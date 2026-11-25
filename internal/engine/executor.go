package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YASSERRMD/FlowForge/internal/db/dbgen"
	"github.com/YASSERRMD/FlowForge/internal/graph"
)

// Execution failure codes recorded on step attempts.
var (
	// ErrUnsupportedKind marks node kinds whose semantics land in later
	// phases (condition/delay in 09, fork/join in 10).
	ErrUnsupportedKind = errors.New("node kind not supported by this engine build")
	// ErrUnknownAction marks an action type missing from the mock registry.
	ErrUnknownAction = errors.New("unknown action type")
	// ErrNodeMissing means the task references a node absent from the
	// published graph — persisted state and graph disagree.
	ErrNodeMissing = errors.New("node not found in published graph")
)

// stepError is a deterministic execution failure: it is recorded on the
// attempt, fails the run and completes the queue task (no task retry).
type stepError struct {
	code    string
	message string
}

func (e *stepError) Error() string { return e.message }

// Executor runs one node of one run inside a single transaction: it locks
// the run, records the attempt, executes the node kind, persists the
// outcome with its events and enqueues successor dispatches.
type Executor struct {
	pool *pgxpool.Pool
}

// NewExecutor builds an Executor over pool.
func NewExecutor(pool *pgxpool.Pool) *Executor {
	return &Executor{pool: pool}
}

// ExecuteNode processes one execute_node task payload. Deterministic step
// failures are persisted (step + run failed) and return nil so the queue
// task completes; only infrastructure errors propagate for task retry.
func (e *Executor) ExecuteNode(ctx context.Context, runID, nodeID string) error {
	rid, err := parseUUID(runID)
	if err != nil {
		return fmt.Errorf("parse run id: %w", err)
	}

	tx, err := e.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := dbgen.New(e.pool).WithTx(tx)

	run, err := q.GetRunForUpdate(ctx, rid)
	if err != nil {
		return fmt.Errorf("load run %s: %w", runID, err)
	}
	if IsTerminalRun(RunStatus(run.Status)) {
		// Replayed dispatch of an already-finished run: nothing to do.
		return commitTx(tx)
	}

	version, err := q.GetWorkflowVersionByID(ctx, run.WorkflowVersionID)
	if err != nil {
		return fmt.Errorf("load version: %w", err)
	}
	var g graph.Graph
	if err := json.Unmarshal(version.GraphJson, &g); err != nil {
		return fmt.Errorf("parse graph: %w", err)
	}
	node, ok := findNode(&g, nodeID)
	if !ok {
		if err := e.failRun(ctx, q, run, nodeID, "node_missing", ErrNodeMissing.Error()); err != nil {
			return err
		}
		return commitTx(tx)
	}

	// First node execution moves the run from queued to running.
	if run.Status == string(RunQueued) {
		if _, err := q.StartRun(ctx, run.ID); err != nil {
			return fmt.Errorf("start run: %w", err)
		}
		if err := appendEvent(ctx, q, run.ID, "run.started", map[string]any{
			"runId": runID,
		}); err != nil {
			return err
		}
		run.Status = string(RunRunning)
	}

	// Record the attempt and mark it running with its input snapshot.
	attempt, err := e.startAttempt(ctx, q, run, node)
	if err != nil {
		return err
	}

	output, execErr := e.executeNode(ctx, q, run, node, &g)
	if execErr != nil {
		var serr *stepError
		if !errors.As(execErr, &serr) {
			serr = &stepError{code: "execution_failed", message: execErr.Error()}
		}
		if err := e.failStep(ctx, q, run, node, attempt, serr); err != nil {
			return err
		}
		return commitTx(tx)
	}

	if err := e.succeedStep(ctx, q, run, node, attempt, output, &g); err != nil {
		return err
	}
	return commitTx(tx)
}

// startAttempt creates and starts the next attempt for the node, recording
// the input snapshot (phase 08: the literal configured params; expression
// resolution arrives in phase 09).
func (e *Executor) startAttempt(ctx context.Context, q *dbgen.Queries,
	run dbgen.WorkflowRun, node graph.Node) (dbgen.StepAttempt, error) {

	next, err := q.NextAttemptNumber(ctx, dbgen.NextAttemptNumberParams{
		RunID:  run.ID,
		NodeID: node.ID,
	})
	if err != nil {
		return dbgen.StepAttempt{}, fmt.Errorf("next attempt number: %w", err)
	}
	attempt, err := q.CreateStepAttempt(ctx, dbgen.CreateStepAttemptParams{
		RunID:         run.ID,
		NodeID:        node.ID,
		NodeType:      string(node.Kind),
		AttemptNumber: next,
	})
	if err != nil {
		return dbgen.StepAttempt{}, fmt.Errorf("create attempt: %w", err)
	}

	input := attemptInput(node)
	attempt, err = q.StartStepAttempt(ctx, dbgen.StartStepAttemptParams{
		ID:            attempt.ID,
		AttemptNumber: next,
		InputJson:     input,
	})
	if err != nil {
		return dbgen.StepAttempt{}, fmt.Errorf("start attempt: %w", err)
	}
	if err := appendEvent(ctx, q, run.ID, "step.started", map[string]any{
		"runId":         uuidOf(run.ID),
		"nodeId":        node.ID,
		"nodeType":      string(node.Kind),
		"attemptNumber": next,
	}); err != nil {
		return dbgen.StepAttempt{}, err
	}
	return attempt, nil
}

// executeNode runs the node's kind-specific behavior and returns its
// output document.
func (e *Executor) executeNode(_ context.Context, _ *dbgen.Queries,
	run dbgen.WorkflowRun, node graph.Node, _ *graph.Graph) (json.RawMessage, error) {

	switch node.Kind {
	case graph.KindStart:
		// No side effects: the start node validates the stored input and
		// hands it to successors as its output.
		var obj map[string]any
		if err := json.Unmarshal(run.Variables, &obj); err != nil || obj == nil {
			return nil, &stepError{code: "invalid_run_input", message: "run input is not a JSON object"}
		}
		return json.RawMessage(run.Variables), nil
	default:
		return nil, &stepError{
			code:    "unsupported_node_kind",
			message: fmt.Sprintf("%s: %s", ErrUnsupportedKind, node.Kind),
		}
	}
}

// succeedStep finishes the attempt, emits step.succeeded and dispatches
// default-handle successors.
func (e *Executor) succeedStep(ctx context.Context, q *dbgen.Queries,
	run dbgen.WorkflowRun, node graph.Node, attempt dbgen.StepAttempt,
	output json.RawMessage, g *graph.Graph) error {

	if _, err := q.FinishStepAttempt(ctx, dbgen.FinishStepAttemptParams{
		ID:         attempt.ID,
		Status:     string(StepSucceeded),
		OutputJson: output,
	}); err != nil {
		return fmt.Errorf("finish attempt: %w", err)
	}
	if err := appendEvent(ctx, q, run.ID, "step.succeeded", map[string]any{
		"runId":         uuidOf(run.ID),
		"nodeId":        node.ID,
		"attemptNumber": attempt.AttemptNumber,
	}); err != nil {
		return err
	}

	for _, edge := range successorEdges(g, node.ID, "") {
		if err := EnqueueStep(ctx, q, uuidOf(run.ID), edge.Target); err != nil {
			return err
		}
	}
	return nil
}

// failStep records the step failure, emits step.failed and fails the run
// (retry policies arrive in phase 11).
func (e *Executor) failStep(ctx context.Context, q *dbgen.Queries,
	run dbgen.WorkflowRun, node graph.Node, attempt dbgen.StepAttempt,
	cause *stepError) error {

	if _, err := q.FinishStepAttempt(ctx, dbgen.FinishStepAttemptParams{
		ID:           attempt.ID,
		Status:       string(StepFailed),
		ErrorCode:    &cause.code,
		ErrorMessage: &cause.message,
	}); err != nil {
		return fmt.Errorf("fail attempt: %w", err)
	}
	if err := appendEvent(ctx, q, run.ID, "step.failed", map[string]any{
		"runId":         uuidOf(run.ID),
		"nodeId":        node.ID,
		"attemptNumber": attempt.AttemptNumber,
		"errorCode":     cause.code,
		"error":         cause.message,
	}); err != nil {
		return err
	}
	return e.finishRun(ctx, q, run, RunFailed, map[string]any{
		"runId":  uuidOf(run.ID),
		"reason": cause.message,
	})
}

// failRun persists a run-level failure when node and state disagree.
func (e *Executor) failRun(ctx context.Context, q *dbgen.Queries,
	run dbgen.WorkflowRun, nodeID, code, message string) error {
	if err := appendEvent(ctx, q, run.ID, "step.failed", map[string]any{
		"runId":     uuidOf(run.ID),
		"nodeId":    nodeID,
		"errorCode": code,
		"error":     message,
	}); err != nil {
		return err
	}
	return e.finishRun(ctx, q, run, RunFailed, map[string]any{
		"runId":  uuidOf(run.ID),
		"reason": message,
	})
}

// finishRun moves the run into a terminal state with its closing event.
func (e *Executor) finishRun(ctx context.Context, q *dbgen.Queries,
	run dbgen.WorkflowRun, status RunStatus, payload map[string]any) error {

	if !CanRunTransition(RunStatus(run.Status), status) {
		return fmt.Errorf("illegal run transition %s -> %s", run.Status, status)
	}
	if _, err := q.FinishRun(ctx, dbgen.FinishRunParams{
		ID:     run.ID,
		Status: string(status),
	}); err != nil {
		return fmt.Errorf("finish run: %w", err)
	}
	return appendEvent(ctx, q, run.ID, "run."+string(status), payload)
}

// attemptInput snapshots the node's configured params as the attempt input.
func attemptInput(node graph.Node) []byte {
	var cfg struct {
		ActionType string          `json:"actionType"`
		Params     json.RawMessage `json:"params"`
	}
	if len(node.Config) > 0 {
		if err := json.Unmarshal(node.Config, &cfg); err == nil && len(cfg.Params) > 0 {
			return cfg.Params
		}
	}
	return []byte(`{}`)
}

func commitTx(tx pgx.Tx) error {
	if err := tx.Commit(context.Background()); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// findNode returns the node with the given ID.
func findNode(g *graph.Graph, id string) (graph.Node, bool) {
	for _, n := range g.Nodes {
		if n.ID == id {
			return n, true
		}
	}
	return graph.Node{}, false
}

// successorEdges returns outgoing edges of nodeID on the given source
// handle ("" selects the default handle: edges without a branch handle).
func successorEdges(g *graph.Graph, nodeID, handle string) []graph.Edge {
	var out []graph.Edge
	for _, e := range g.Edges {
		if e.Source != nodeID {
			continue
		}
		edgeHandle := ""
		if e.SourceHandle != nil {
			edgeHandle = *e.SourceHandle
		}
		if handle == "" && (edgeHandle == "" || edgeHandle == "default") {
			out = append(out, e)
		} else if handle != "" && edgeHandle == handle {
			out = append(out, e)
		}
	}
	return out
}

// appendEvent writes one run event and its outbox mirror in the caller's
// transaction.
func appendEvent(ctx context.Context, q *dbgen.Queries, runID pgtype.UUID,
	eventType string, payload map[string]any) error {

	data := mustJSON(payload)
	if _, err := q.AppendRunEvent(ctx, dbgen.AppendRunEventParams{
		RunID:     runID,
		EventType: eventType,
		Payload:   data,
	}); err != nil {
		return fmt.Errorf("append %s: %w", eventType, err)
	}
	if _, err := q.EnqueueOutbox(ctx, dbgen.EnqueueOutboxParams{
		Topic: TopicRunEvents,
		Payload: mustJSON(map[string]any{
			"type":  eventType,
			"runId": uuidOf(runID),
			"data":  json.RawMessage(data),
		}),
	}); err != nil {
		return fmt.Errorf("outbox %s: %w", eventType, err)
	}
	return nil
}
