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
	"github.com/YASSERRMD/FlowForge/internal/graph"
)

// Run initiation errors mapped by the HTTP layer.
var (
	// ErrWorkflowNotFound means the workflow does not exist or is not
	// visible to the caller.
	ErrWorkflowNotFound = errors.New("workflow not found")
	// ErrNoPublishedVersion means the workflow has never been published;
	// only immutable published graphs may run.
	ErrNoPublishedVersion = errors.New("workflow has no published version")
	// ErrInvalidInput means the run input is not a JSON object.
	ErrInvalidInput = errors.New("run input must be a JSON object")
)

// TaskExecuteNode is the durable-queue task type workers claim to execute
// one graph node inside a run.
const TaskExecuteNode = "execute_node"

// TopicRunEvents is the outbox topic carrying run lifecycle events for
// relay (WebSocket fan-out lands in phase 12).
const TopicRunEvents = "run.events"

// maxInputBytes bounds the normalized run input document.
const maxInputBytes = 64 * 1024

// maxTaskAttempts bounds how often the queue retries a node dispatch
// before marking the task dead.
const maxTaskAttempts = 8

// CreatedRun is the result of run initiation.
type CreatedRun struct {
	Run          dbgen.WorkflowRun
	Version      dbgen.WorkflowVersion
	StartNodeID  string
	ReplayedFrom bool // true when idempotencyKey matched an earlier run
}

// Service owns run initiation: validating the published version,
// normalizing input and atomically persisting the queued run plus its
// first dispatch.
type Service struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

// NewService builds a Service over pool.
func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool, now: time.Now}
}

func parseUUID(s string) (pgtype.UUID, error) {
	var id pgtype.UUID
	if err := id.Scan(s); err != nil {
		return id, fmt.Errorf("parse uuid %q: %w", s, err)
	}
	return id, nil
}

func uuidOf(id pgtype.UUID) string {
	if !id.Valid {
		return ""
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		id.Bytes[0:4], id.Bytes[4:6], id.Bytes[6:8], id.Bytes[8:10], id.Bytes[10:16])
}

// StepTaskDedupeKey is the idempotency key of one logical node dispatch:
// re-enqueues of the same node execution collapse onto one queue row.
func StepTaskDedupeKey(runID, nodeID string) string {
	return "run:" + runID + ":node:" + nodeID
}

// normalizeInput validates and canonicalizes the caller-supplied run input.
// Absent input becomes an empty object; anything that is not a JSON object
// is rejected so downstream context lookups have a stable shape.
func normalizeInput(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 {
		return json.RawMessage(`{}`), nil
	}
	if len(raw) > maxInputBytes {
		return nil, fmt.Errorf("%w: exceeds %d bytes", ErrInvalidInput, maxInputBytes)
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	if obj == nil {
		return nil, ErrInvalidInput
	}
	canonical, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	return canonical, nil
}

// CreateRun atomically creates a queued run of the workflow's published
// version, appends its first event and outbox message, and enqueues the
// start node dispatch — all in one transaction. A repeated idempotencyKey
// returns the earlier run instead of creating a duplicate.
func (s *Service) CreateRun(ctx context.Context, callerID, workflowID string,
	input json.RawMessage, idempotencyKey string) (*CreatedRun, error) {

	wfID, err := parseUUID(workflowID)
	if err != nil {
		return nil, ErrWorkflowNotFound
	}
	caller, err := parseUUID(callerID)
	if err != nil {
		return nil, fmt.Errorf("parse caller: %w", err)
	}

	q := dbgen.New(s.pool)
	wf, err := q.GetWorkflow(ctx, wfID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrWorkflowNotFound
		}
		return nil, fmt.Errorf("load workflow: %w", err)
	}
	if wf.OwnerID != caller {
		// Fail closed like a missing row: existence must not leak.
		return nil, ErrWorkflowNotFound
	}

	version, err := q.GetPublishedVersion(ctx, wfID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNoPublishedVersion
		}
		return nil, fmt.Errorf("load published version: %w", err)
	}

	var g graph.Graph
	if err := json.Unmarshal(version.GraphJson, &g); err != nil {
		return nil, fmt.Errorf("parse published graph: %w", err)
	}
	startNode, err := startNodeOf(&g)
	if err != nil {
		return nil, err
	}

	variables, err := normalizeInput(input)
	if err != nil {
		return nil, err
	}

	var key *string
	if idempotencyKey != "" {
		key = &idempotencyKey
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := q.WithTx(tx)

	run, err := qtx.CreateRun(ctx, dbgen.CreateRunParams{
		WorkflowID:        wfID,
		WorkflowVersionID: version.ID,
		Variables:         variables,
		IdempotencyKey:    key,
		TriggeredBy:       caller,
		TriggerType:       "manual",
	})
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("create run: %w", err)
		}
		// Idempotent replay: the key already exists, return that run.
		existing, gerr := q.GetRunByIdempotencyKey(ctx, dbgen.GetRunByIdempotencyKeyParams{
			WorkflowID:     wfID,
			IdempotencyKey: key,
		})
		if gerr != nil {
			return nil, fmt.Errorf("load idempotent run: %w", gerr)
		}
		return &CreatedRun{Run: existing, Version: version, StartNodeID: startNode.ID, ReplayedFrom: true}, nil
	}

	if err := s.appendRunCreated(ctx, qtx, run, version); err != nil {
		return nil, err
	}
	if err := EnqueueStep(ctx, qtx, uuidOf(run.ID), startNode.ID); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit run creation: %w", err)
	}
	return &CreatedRun{Run: run, Version: version, StartNodeID: startNode.ID}, nil
}

// appendRunCreated writes the run.created event and its outbox mirror.
func (s *Service) appendRunCreated(ctx context.Context, q *dbgen.Queries,
	run dbgen.WorkflowRun, version dbgen.WorkflowVersion) error {

	payload := mustJSON(map[string]any{
		"runId":         uuidOf(run.ID),
		"workflowId":    uuidOf(run.WorkflowID),
		"versionNumber": version.VersionNumber,
		"triggeredBy":   uuidOf(run.TriggeredBy),
	})
	if _, err := q.AppendRunEvent(ctx, dbgen.AppendRunEventParams{
		RunID:     run.ID,
		EventType: "run.created",
		Payload:   payload,
	}); err != nil {
		return fmt.Errorf("append run.created: %w", err)
	}
	if _, err := q.EnqueueOutbox(ctx, dbgen.EnqueueOutboxParams{
		Topic: TopicRunEvents,
		Payload: mustJSON(map[string]any{
			"type":  "run.created",
			"runId": uuidOf(run.ID),
			"data":  json.RawMessage(payload),
		}),
	}); err != nil {
		return fmt.Errorf("enqueue outbox: %w", err)
	}
	return nil
}

// startNodeOf returns the graph's single start node.
func startNodeOf(g *graph.Graph) (graph.Node, error) {
	for _, n := range g.Nodes {
		if n.Kind == graph.KindStart {
			return n, nil
		}
	}
	return graph.Node{}, fmt.Errorf("graph has no start node")
}

func mustJSON(v any) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("marshal static payload: %v", err))
	}
	return data
}

// AttemptView is the API view of one step attempt.
type AttemptView struct {
	AttemptNumber int32           `json:"attemptNumber"`
	Status        string          `json:"status"`
	Input         json.RawMessage `json:"input,omitempty"`
	Output        json.RawMessage `json:"output,omitempty"`
	ErrorCode     string          `json:"errorCode,omitempty"`
	ErrorMessage  string          `json:"errorMessage,omitempty"`
	StartedAt     string          `json:"startedAt,omitempty"`
	FinishedAt    string          `json:"finishedAt,omitempty"`
}

// NodeState is the latest durable state of one graph node in a run.
type NodeState struct {
	NodeID   string        `json:"nodeId"`
	NodeType string        `json:"nodeType"`
	Status   string        `json:"status"`
	Attempts []AttemptView `json:"attempts"`
}

// RunDetail is the read model for the run detail endpoint.
type RunDetail struct {
	ID            string          `json:"id"`
	WorkflowID    string          `json:"workflowId"`
	VersionNumber int32           `json:"versionNumber"`
	Status        string          `json:"status"`
	TriggerType   string          `json:"triggerType"`
	Variables     json.RawMessage `json:"variables"`
	StartedAt     string          `json:"startedAt,omitempty"`
	FinishedAt    string          `json:"finishedAt,omitempty"`
	DurationMs    int64           `json:"durationMs,omitempty"`
	Nodes         []NodeState     `json:"nodes"`
	CreatedAt     string          `json:"createdAt"`
}

// ErrRunNotFound means the run does not exist or is not visible to the
// caller.
var ErrRunNotFound = errors.New("run not found")

// GetRunDetail returns the read model for one run, scoped to its owner.
func (s *Service) GetRunDetail(ctx context.Context, callerID, runID string) (*RunDetail, error) {
	rid, err := parseUUID(runID)
	if err != nil {
		return nil, ErrRunNotFound
	}
	caller, err := parseUUID(callerID)
	if err != nil {
		return nil, fmt.Errorf("parse caller: %w", err)
	}
	q := dbgen.New(s.pool)
	run, err := q.GetRun(ctx, rid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRunNotFound
		}
		return nil, fmt.Errorf("load run: %w", err)
	}
	wf, err := q.GetWorkflow(ctx, run.WorkflowID)
	if err != nil {
		return nil, fmt.Errorf("load workflow: %w", err)
	}
	if wf.OwnerID != caller {
		return nil, ErrRunNotFound
	}
	version, err := q.GetWorkflowVersionByID(ctx, run.WorkflowVersionID)
	if err != nil {
		return nil, fmt.Errorf("load version: %w", err)
	}
	attempts, err := q.ListStepAttemptsByRun(ctx, rid)
	if err != nil {
		return nil, fmt.Errorf("list attempts: %w", err)
	}

	// Group attempts by node, preserving first-seen node order.
	order := []string{}
	byNode := map[string]*NodeState{}
	for _, a := range attempts {
		ns, ok := byNode[a.NodeID]
		if !ok {
			ns = &NodeState{NodeID: a.NodeID, NodeType: a.NodeType, Status: a.Status}
			byNode[a.NodeID] = ns
			order = append(order, a.NodeID)
		}
		ns.Attempts = append(ns.Attempts, AttemptView{
			AttemptNumber: a.AttemptNumber,
			Status:        a.Status,
			Input:         a.InputJson,
			Output:        a.OutputJson,
			ErrorCode:     deref(a.ErrorCode),
			ErrorMessage:  deref(a.ErrorMessage),
			StartedAt:     formatTS(a.StartedAt),
			FinishedAt:    formatTS(a.FinishedAt),
		})
		ns.Status = a.Status // attempts arrive in attempt order; latest wins
	}
	nodes := make([]NodeState, 0, len(order))
	for _, id := range order {
		nodes = append(nodes, *byNode[id])
	}

	detail := &RunDetail{
		ID:            uuidOf(run.ID),
		WorkflowID:    uuidOf(run.WorkflowID),
		VersionNumber: version.VersionNumber,
		Status:        run.Status,
		TriggerType:   run.TriggerType,
		Variables:     run.Variables,
		StartedAt:     formatTS(run.StartedAt),
		FinishedAt:    formatTS(run.FinishedAt),
		Nodes:         nodes,
		CreatedAt:     run.CreatedAt.Time.UTC().Format(tsLayout),
	}
	if run.StartedAt.Valid {
		end := run.FinishedAt.Time
		if !run.FinishedAt.Valid {
			end = s.now()
		}
		detail.DurationMs = end.Sub(run.StartedAt.Time).Milliseconds()
	}
	return detail, nil
}

// RunContextView is the read-only expression context of one run. Steps map
// node IDs to their latest succeeded output document.
type RunContextView struct {
	Version int                        `json:"version"`
	Trigger json.RawMessage            `json:"trigger"`
	Steps   map[string]json.RawMessage `json:"steps"`
}

// GetRunContext returns the typed variable context used for expressions,
// scoped to the run owner.
func (s *Service) GetRunContext(ctx context.Context, callerID, runID string) (*RunContextView, error) {
	run, err := s.ownedRun(ctx, callerID, runID)
	if err != nil {
		return nil, err
	}
	q := dbgen.New(s.pool)
	outputs, err := q.ListSucceededStepOutputsByRun(ctx, run.ID)
	if err != nil {
		return nil, fmt.Errorf("list step outputs: %w", err)
	}
	steps := make(map[string]json.RawMessage, len(outputs))
	for _, o := range outputs {
		steps[o.NodeID] = o.OutputJson
	}
	trigger := run.Variables
	if len(trigger) == 0 {
		trigger = json.RawMessage(`{}`)
	}
	return &RunContextView{Version: 1, Trigger: trigger, Steps: steps}, nil
}

// NotificationView is the API view of one persisted in-app notification.
type NotificationView struct {
	ID        string          `json:"id"`
	NodeID    string          `json:"nodeId"`
	Channel   string          `json:"channel"`
	Recipient string          `json:"recipient,omitempty"`
	Subject   string          `json:"subject,omitempty"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt string          `json:"createdAt"`
}

// ListRunNotifications returns the in-app notifications a run produced,
// scoped to the run owner.
func (s *Service) ListRunNotifications(ctx context.Context, callerID, runID string) ([]NotificationView, error) {
	run, err := s.ownedRun(ctx, callerID, runID)
	if err != nil {
		return nil, err
	}
	rows, err := dbgen.New(s.pool).ListNotificationsByRun(ctx, run.ID)
	if err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}
	out := make([]NotificationView, 0, len(rows))
	for _, n := range rows {
		out = append(out, NotificationView{
			ID:        uuidOf(n.ID),
			NodeID:    n.NodeID,
			Channel:   n.Channel,
			Recipient: deref(n.Recipient),
			Subject:   deref(n.Subject),
			Payload:   n.Payload,
			CreatedAt: formatTS(n.CreatedAt),
		})
	}
	return out, nil
}

// ownedRun loads a run and verifies the caller owns its workflow. Missing and
// foreign runs both map to ErrRunNotFound so IDs cannot be probed.
func (s *Service) ownedRun(ctx context.Context, callerID, runID string) (dbgen.WorkflowRun, error) {
	rid, err := parseUUID(runID)
	if err != nil {
		return dbgen.WorkflowRun{}, ErrRunNotFound
	}
	caller, err := parseUUID(callerID)
	if err != nil {
		return dbgen.WorkflowRun{}, fmt.Errorf("parse caller: %w", err)
	}
	q := dbgen.New(s.pool)
	run, err := q.GetRun(ctx, rid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dbgen.WorkflowRun{}, ErrRunNotFound
		}
		return dbgen.WorkflowRun{}, fmt.Errorf("load run: %w", err)
	}
	wf, err := q.GetWorkflow(ctx, run.WorkflowID)
	if err != nil {
		return dbgen.WorkflowRun{}, fmt.Errorf("load workflow: %w", err)
	}
	if wf.OwnerID != caller {
		return dbgen.WorkflowRun{}, ErrRunNotFound
	}
	return run, nil
}

const tsLayout = "2006-01-02T15:04:05.999999999Z07:00"

func formatTS(ts pgtype.Timestamptz) string {
	if !ts.Valid {
		return ""
	}
	return ts.Time.UTC().Format(tsLayout)
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
