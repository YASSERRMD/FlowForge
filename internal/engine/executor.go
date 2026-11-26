package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YASSERRMD/FlowForge/internal/db/dbgen"
	"github.com/YASSERRMD/FlowForge/internal/engine/actions"
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

// maxAttemptPayloadBytes bounds per-attempt input/output documents so a
// runaway action cannot write unbounded payloads into step_attempts.
const maxAttemptPayloadBytes = 256 * 1024

// Executor runs one node of one run inside a single transaction: it locks
// the run, records the attempt, executes the node kind, persists the
// outcome with its events and enqueues successor dispatches.
type Executor struct {
	pool    *pgxpool.Pool
	actions *actions.Registry
	now     func() time.Time
}

// NewExecutor builds an Executor with the default deterministic action
// registry.
func NewExecutor(pool *pgxpool.Pool) *Executor {
	return NewExecutorWithActions(pool, actions.Default())
}

// NewExecutorWithActions builds an Executor with an explicit registry
// (tests inject fakes).
func NewExecutorWithActions(pool *pgxpool.Pool, registry *actions.Registry) *Executor {
	return &Executor{pool: pool, actions: registry, now: time.Now}
}

// WithClock overrides the executor clock (tests use a fixed instant).
func (e *Executor) WithClock(now func() time.Time) *Executor {
	e.now = now
	return e
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

	rc, err := e.buildContext(ctx, q, run)
	if err != nil {
		return err
	}

	// Record the attempt and mark it running with its resolved input snapshot.
	attempt, err := e.startAttempt(ctx, q, run, node, rc)
	if err != nil {
		var serr *stepError
		if errors.As(err, &serr) {
			if ferr := e.failRun(ctx, q, run, node.ID, serr.code, serr.message); ferr != nil {
				return ferr
			}
			return commitTx(tx)
		}
		return err
	}

	result, execErr := e.executeNode(ctx, run, node, rc)
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

	if err := e.succeedStep(ctx, q, run, node, attempt, result, &g); err != nil {
		return err
	}
	return commitTx(tx)
}

// buildContext assembles the expression context from the run input and the
// latest succeeded output of each node.
func (e *Executor) buildContext(ctx context.Context, q *dbgen.Queries,
	run dbgen.WorkflowRun) (*graph.Context, error) {

	rc, err := graph.NewContext(run.Variables)
	if err != nil {
		return nil, fmt.Errorf("build run context: %w", err)
	}
	outputs, err := q.ListSucceededStepOutputsByRun(ctx, run.ID)
	if err != nil {
		return nil, fmt.Errorf("load step outputs: %w", err)
	}
	for _, out := range outputs {
		if err := rc.MergeStep(out.NodeID, out.OutputJson); err != nil {
			return nil, err
		}
	}
	return rc, nil
}

// startAttempt creates and starts the next attempt for the node, recording
// the resolved input snapshot.
func (e *Executor) startAttempt(ctx context.Context, q *dbgen.Queries,
	run dbgen.WorkflowRun, node graph.Node, rc *graph.Context) (dbgen.StepAttempt, error) {

	input, err := attemptInput(node, rc)
	if err != nil {
		return dbgen.StepAttempt{}, &stepError{code: "invalid_params", message: err.Error()}
	}
	if len(input) > maxAttemptPayloadBytes {
		// Corrupt configuration, not runtime data: fail the run without an
		// attempt rather than record a poisoned input snapshot.
		return dbgen.StepAttempt{}, &stepError{code: "input_too_large",
			message: fmt.Sprintf("attempt input exceeds %d bytes", maxAttemptPayloadBytes)}
	}
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

// nodeResult is the outcome of one node kind: its output document, the
// selected condition branch (empty for non-branching kinds) and an optional
// earliest dispatch time (delay nodes).
type nodeResult struct {
	output json.RawMessage
	branch string
	runAt  pgtype.Timestamptz
}

// executeNode runs the node's kind-specific behavior and returns its result.
func (e *Executor) executeNode(ctx context.Context, run dbgen.WorkflowRun,
	node graph.Node, rc *graph.Context) (nodeResult, error) {

	switch node.Kind {
	case graph.KindStart:
		// No side effects: the start node validates the stored input and
		// hands it to successors as its output.
		var obj map[string]any
		if err := json.Unmarshal(run.Variables, &obj); err != nil || obj == nil {
			return nodeResult{}, &stepError{code: "invalid_run_input", message: "run input is not a JSON object"}
		}
		return nodeResult{output: json.RawMessage(run.Variables)}, nil
	case graph.KindAction:
		out, err := e.executeAction(ctx, node, rc)
		return nodeResult{output: out}, err
	case graph.KindCondition:
		return e.executeCondition(node, rc)
	case graph.KindDelay:
		return e.executeDelay(node)
	case graph.KindEnd:
		// The end node carries no behavior; completion is decided after the
		// attempt closes, when durable state is checked.
		return nodeResult{output: json.RawMessage(`{}`)}, nil
	default:
		return nodeResult{}, &stepError{
			code:    "unsupported_node_kind",
			message: fmt.Sprintf("%s: %s", ErrUnsupportedKind, node.Kind),
		}
	}
}

// conditionConfig is the persisted config of a condition node.
type conditionConfig struct {
	Expression string `json:"expression"`
}

// executeCondition evaluates the node expression against the run context and
// selects the true or false branch. Evaluation is pure and side-effect free.
func (e *Executor) executeCondition(node graph.Node, rc *graph.Context) (nodeResult, error) {
	var cfg conditionConfig
	if len(node.Config) > 0 {
		if err := json.Unmarshal(node.Config, &cfg); err != nil {
			return nodeResult{}, &stepError{code: "invalid_condition_config", message: err.Error()}
		}
	}
	result, err := graph.EvaluateExpression(cfg.Expression, rc)
	if err != nil {
		return nodeResult{}, &stepError{code: "expression_error", message: err.Error()}
	}
	branch := "false"
	if result {
		branch = "true"
	}
	return nodeResult{
		output: mustJSON(map[string]any{
			"result":         result,
			"branch":         branch,
			"contextVersion": rc.Version,
		}),
		branch: branch,
	}, nil
}

// delayConfig is the persisted config of a delay node.
type delayConfig struct {
	DurationMs int64 `json:"durationMs"`
}

// executeDelay converts a delay into a persisted wake-up time: the node
// succeeds immediately and successors are scheduled at that instant, so no
// worker goroutine sleeps and the wait survives restarts.
func (e *Executor) executeDelay(node graph.Node) (nodeResult, error) {
	var cfg delayConfig
	if len(node.Config) > 0 {
		if err := json.Unmarshal(node.Config, &cfg); err != nil {
			return nodeResult{}, &stepError{code: "invalid_delay_config", message: err.Error()}
		}
	}
	if cfg.DurationMs < 1 {
		return nodeResult{}, &stepError{code: "invalid_delay_config", message: "durationMs must be >= 1"}
	}
	clock := e.now
	if clock == nil {
		clock = time.Now
	}
	wakeAt := clock().UTC().Add(time.Duration(cfg.DurationMs) * time.Millisecond)
	return nodeResult{
		output: mustJSON(map[string]any{
			"durationMs": cfg.DurationMs,
			"wakeAt":     wakeAt.Format(time.RFC3339Nano),
		}),
		runAt: pgtype.Timestamptz{Time: wakeAt, Valid: true},
	}, nil
}

// actionConfig is the persisted config of an action node.
type actionConfig struct {
	ActionType string          `json:"actionType"`
	Params     json.RawMessage `json:"params"`
}

// executeAction runs the node's mock action through the registry. Unknown
// types and action failures are deterministic step failures. Params are
// resolved against the run context before execution.
func (e *Executor) executeAction(ctx context.Context, node graph.Node, rc *graph.Context) (json.RawMessage, error) {
	var cfg actionConfig
	if len(node.Config) > 0 {
		if err := json.Unmarshal(node.Config, &cfg); err != nil {
			return nil, &stepError{code: "invalid_action_config", message: err.Error()}
		}
	}
	params, err := resolveParams(cfg.Params, rc)
	if err != nil {
		return nil, &stepError{code: "invalid_params", message: err.Error()}
	}
	out, err := e.actions.Execute(ctx, cfg.ActionType, params)
	if err != nil {
		if errors.Is(err, actions.ErrUnknownType) {
			return nil, &stepError{code: "unknown_action", message: err.Error()}
		}
		return nil, &stepError{code: "action_failed", message: err.Error()}
	}
	return out, nil
}

// succeedStep finishes the attempt, emits step.succeeded and dispatches
// successors according to the node's routing (default handle, selected
// condition branch, or delayed wake-up).
func (e *Executor) succeedStep(ctx context.Context, q *dbgen.Queries,
	run dbgen.WorkflowRun, node graph.Node, attempt dbgen.StepAttempt,
	result nodeResult, g *graph.Graph) error {

	if len(result.output) > maxAttemptPayloadBytes {
		return e.failStep(ctx, q, run, node, attempt, &stepError{
			code:    "output_too_large",
			message: fmt.Sprintf("action output exceeds %d bytes", maxAttemptPayloadBytes),
		})
	}
	if _, err := q.FinishStepAttempt(ctx, dbgen.FinishStepAttemptParams{
		ID:         attempt.ID,
		Status:     string(StepSucceeded),
		OutputJson: result.output,
	}); err != nil {
		return fmt.Errorf("finish attempt: %w", err)
	}
	payload := map[string]any{
		"runId":         uuidOf(run.ID),
		"nodeId":        node.ID,
		"attemptNumber": attempt.AttemptNumber,
	}
	if node.Kind == graph.KindCondition {
		payload["branch"] = result.branch
	}
	if err := appendEvent(ctx, q, run.ID, "step.succeeded", payload); err != nil {
		return err
	}

	if node.Kind == graph.KindEnd {
		return e.maybeCompleteRun(ctx, q, run, node.ID)
	}

	if node.Kind == graph.KindCondition {
		return e.routeCondition(ctx, q, run, node, result.branch, g)
	}

	for _, edge := range successorEdges(g, node.ID, "") {
		if err := EnqueueStepAt(ctx, q, uuidOf(run.ID), edge.Target, result.runAt); err != nil {
			return err
		}
	}
	return nil
}

// routeCondition dispatches the selected branch and propagates skips down the
// branch that was not taken.
func (e *Executor) routeCondition(ctx context.Context, q *dbgen.Queries,
	run dbgen.WorkflowRun, node graph.Node, selected string, g *graph.Graph) error {

	other := "false"
	if selected == "false" {
		other = "true"
	}
	for _, edge := range successorEdges(g, node.ID, selected) {
		if err := EnqueueStep(ctx, q, uuidOf(run.ID), edge.Target); err != nil {
			return err
		}
	}
	for _, edge := range successorEdges(g, node.ID, other) {
		if err := e.propagateSkips(ctx, q, run, g, edge.Target, "branch_not_taken"); err != nil {
			return err
		}
	}
	return nil
}

// propagateSkips marks the subtree reachable only through an untaken branch
// as skipped. A node is skipped only when every one of its predecessors is
// skipped (or absent); shared join nodes stay active for the taken path.
func (e *Executor) propagateSkips(ctx context.Context, q *dbgen.Queries,
	run dbgen.WorkflowRun, g *graph.Graph, rootID, reason string) error {

	// If the root already ran or is queued from another active path, leave
	// it alone: that path owns the node.
	rootKey := StepTaskDedupeKey(uuidOf(run.ID), rootID)
	pending, err := q.CountPendingNodeTasks(ctx, rootKey)
	if err != nil {
		return fmt.Errorf("count pending node: %w", err)
	}
	if pending > 0 {
		return nil
	}
	statuses, err := q.ListLatestNodeStatusesByRun(ctx, run.ID)
	if err != nil {
		return fmt.Errorf("load node statuses: %w", err)
	}
	latest := make(map[string]string, len(statuses))
	for _, s := range statuses {
		latest[s.NodeID] = s.Status
	}
	switch latest[rootID] {
	case string(StepSucceeded), string(StepRunning), string(StepFailed), string(StepCanceled):
		return nil
	}

	descendants := descendantsOf(g, rootID)
	inactive := map[string]bool{rootID: true}

	// Fixed point: a descendant becomes inactive when every predecessor is
	// inactive or already skipped. This correctly keeps shared joins active.
	for changed := true; changed; {
		changed = false
		for _, id := range descendants {
			if id == rootID || inactive[id] {
				continue
			}
			preds := predecessorIDs(g, id)
			if len(preds) == 0 {
				continue
			}
			allInactive := true
			for _, pred := range preds {
				if !inactive[pred] && latest[pred] != string(StepSkipped) {
					allInactive = false
					break
				}
			}
			if allInactive {
				inactive[id] = true
				changed = true
			}
		}
	}

	toSkip := append([]string{rootID}, descendants...)
	for _, id := range toSkip {
		if !inactive[id] || latest[id] == string(StepSkipped) {
			continue
		}
		if err := e.skipNode(ctx, q, run, g, id, reason); err != nil {
			return err
		}
	}
	return nil
}

// skipNode records one skipped attempt and its event.
func (e *Executor) skipNode(ctx context.Context, q *dbgen.Queries,
	run dbgen.WorkflowRun, g *graph.Graph, nodeID, reason string) error {

	node, ok := findNode(g, nodeID)
	if !ok {
		return nil
	}
	next, err := q.NextAttemptNumber(ctx, dbgen.NextAttemptNumberParams{
		RunID:  run.ID,
		NodeID: nodeID,
	})
	if err != nil {
		return fmt.Errorf("next attempt number: %w", err)
	}
	if _, err := q.CreateSkippedStepAttempt(ctx, dbgen.CreateSkippedStepAttemptParams{
		RunID:         run.ID,
		NodeID:        nodeID,
		NodeType:      string(node.Kind),
		AttemptNumber: next,
		OutputJson:    mustJSON(map[string]any{"skipped": true, "reason": reason}),
	}); err != nil {
		return fmt.Errorf("skip attempt: %w", err)
	}
	if err := appendEvent(ctx, q, run.ID, "step.skipped", map[string]any{
		"runId":    uuidOf(run.ID),
		"nodeId":   nodeID,
		"reason":   reason,
		"attemptN": next,
	}); err != nil {
		return err
	}
	return nil
}

// descendantsOf returns every node reachable from rootID (excluding root).
func descendantsOf(g *graph.Graph, rootID string) []string {
	seen := map[string]bool{rootID: true}
	var order []string
	queue := []string{rootID}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		for _, edge := range g.Edges {
			if edge.Source != id || seen[edge.Target] {
				continue
			}
			seen[edge.Target] = true
			order = append(order, edge.Target)
			queue = append(queue, edge.Target)
		}
	}
	return order
}

func predecessorIDs(g *graph.Graph, id string) []string {
	var preds []string
	for _, edge := range g.Edges {
		if edge.Target == id {
			preds = append(preds, edge.Source)
		}
	}
	return preds
}

// maybeCompleteRun finishes the run only when durable state agrees the
// graph is done: no active step attempts and no pending dispatches remain.
func (e *Executor) maybeCompleteRun(ctx context.Context, q *dbgen.Queries,
	run dbgen.WorkflowRun, endNodeID string) error {

	active, err := q.CountActiveStepAttempts(ctx, run.ID)
	if err != nil {
		return fmt.Errorf("count active steps: %w", err)
	}
	pending, err := q.CountPendingStepTasks(ctx, dbgen.CountPendingStepTasksParams{
		DedupeKey:   "run:" + uuidOf(run.ID) + ":%",
		DedupeKey_2: StepTaskDedupeKey(uuidOf(run.ID), endNodeID),
	})
	if err != nil {
		return fmt.Errorf("count pending tasks: %w", err)
	}
	if active > 0 || pending > 0 {
		// Other branches are still in flight; the last end node to finish
		// with a quiet queue completes the run.
		return nil
	}
	return e.finishRun(ctx, q, run, RunSucceeded, map[string]any{
		"runId": uuidOf(run.ID),
	})
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

// attemptInput snapshots the node's resolved input. Actions record their
// resolved params; conditions and delays record their configuration.
func attemptInput(node graph.Node, rc *graph.Context) ([]byte, error) {
	switch node.Kind {
	case graph.KindAction:
		var cfg actionConfig
		if len(node.Config) > 0 {
			if err := json.Unmarshal(node.Config, &cfg); err != nil {
				return nil, err
			}
		}
		return resolveParams(cfg.Params, rc)
	case graph.KindCondition, graph.KindDelay:
		if len(node.Config) == 0 {
			return []byte(`{}`), nil
		}
		var obj map[string]any
		if err := json.Unmarshal(node.Config, &obj); err != nil {
			return nil, err
		}
		return json.Marshal(obj)
	default:
		return []byte(`{}`), nil
	}
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
