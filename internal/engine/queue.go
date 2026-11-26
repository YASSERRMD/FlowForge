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
)

// Queue is the typed worker-side view of the durable scheduled_tasks queue.
// Claims use SELECT ... FOR UPDATE SKIP LOCKED with bounded leases: a
// crashed worker's tasks become claimable again when the lease expires.
type Queue struct {
	pool *pgxpool.Pool
}

// NewQueue builds a Queue over pool.
func NewQueue(pool *pgxpool.Pool) *Queue { return &Queue{pool: pool} }

// ClaimedTask is one leased queue row.
type ClaimedTask struct {
	ID       string
	Type     string
	Dedupe   string
	Payload  json.RawMessage
	Attempts int32
}

// StepTaskPayload is the payload of an execute_node task.
type StepTaskPayload struct {
	RunID  string `json:"runId"`
	NodeID string `json:"nodeId"`
}

// EnqueueStep inserts a node dispatch into the durable queue inside the
// caller's transaction. The dedupe key collapses duplicate enqueues of the
// same logical node execution onto one row.
func EnqueueStep(ctx context.Context, q *dbgen.Queries, runID, nodeID string) error {
	return enqueueStep(ctx, q, runID, nodeID, pgtype.Timestamptz{})
}

// EnqueueStepAt schedules a node dispatch to become runnable no earlier than
// runAt. It is used by delay nodes so waiting survives worker restarts
// without sleeping goroutines.
func EnqueueStepAt(ctx context.Context, q *dbgen.Queries, runID, nodeID string, runAt pgtype.Timestamptz) error {
	return enqueueStep(ctx, q, runID, nodeID, runAt)
}

func enqueueStep(ctx context.Context, q *dbgen.Queries, runID, nodeID string, runAt pgtype.Timestamptz) error {
	_, err := q.EnqueueTask(ctx, dbgen.EnqueueTaskParams{
		TaskType:    TaskExecuteNode,
		DedupeKey:   StepTaskDedupeKey(runID, nodeID),
		Payload:     mustJSON(StepTaskPayload{RunID: runID, NodeID: nodeID}),
		Column4:     runAt,
		MaxAttempts: maxTaskAttempts,
	})
	// ON CONFLICT DO NOTHING yields no row: the dispatch already exists.
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("enqueue step %s/%s: %w", runID, nodeID, err)
	}
	return nil
}

// Claim leases up to max runnable tasks for workerID. Rows locked by other
// workers are skipped; expired leases are reclaimed. leaseSeconds bounds how
// long a crashed worker may hold a task before it returns to the queue.
func (q *Queue) Claim(ctx context.Context, workerID string, max int32, leaseSeconds int32) ([]ClaimedTask, error) {
	rows, err := dbgen.New(q.pool).ClaimTasks(ctx, dbgen.ClaimTasksParams{
		LeasedBy: &workerID,
		Limit:    max,
		Secs:     float64(leaseSeconds),
	})
	if err != nil {
		return nil, fmt.Errorf("claim tasks: %w", err)
	}
	tasks := make([]ClaimedTask, 0, len(rows))
	for _, r := range rows {
		tasks = append(tasks, ClaimedTask{
			ID:       uuidOf(r.ID),
			Type:     r.TaskType,
			Dedupe:   r.DedupeKey,
			Payload:  r.Payload,
			Attempts: r.Attempts,
		})
	}
	return tasks, nil
}

// Renew extends a live lease while the worker is still executing.
func (q *Queue) Renew(ctx context.Context, taskID, workerID string, leaseSeconds int32) error {
	id, err := parseUUID(taskID)
	if err != nil {
		return err
	}
	_, err = dbgen.New(q.pool).RenewTaskLease(ctx, dbgen.RenewTaskLeaseParams{
		ID:       id,
		LeasedBy: &workerID,
		Secs:     float64(leaseSeconds),
	})
	return err
}

// Complete marks a leased task done.
func (q *Queue) Complete(ctx context.Context, taskID, workerID string) error {
	id, err := parseUUID(taskID)
	if err != nil {
		return err
	}
	return dbgen.New(q.pool).CompleteTask(ctx, dbgen.CompleteTaskParams{
		ID:       id,
		LeasedBy: &workerID,
	})
}

// Fail returns the task to pending with a backoff delay, or marks it dead
// once its attempt budget is exhausted.
func (q *Queue) Fail(ctx context.Context, taskID, workerID, message string, backoffSeconds int32) error {
	id, err := parseUUID(taskID)
	if err != nil {
		return err
	}
	_, err = dbgen.New(q.pool).FailTask(ctx, dbgen.FailTaskParams{
		ID:        id,
		LeasedBy:  &workerID,
		LastError: &message,
		Secs:      float64(backoffSeconds),
	})
	return err
}
