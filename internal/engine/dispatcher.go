package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Dispatcher routes claimed tasks to the executor by task type.
type Dispatcher struct {
	executor *Executor
	log      *slog.Logger
}

// NewDispatcher builds a Dispatcher whose execute_node tasks run through
// the workflow executor.
func NewDispatcher(pool *pgxpool.Pool, log *slog.Logger) *Dispatcher {
	return &Dispatcher{executor: NewExecutor(pool), log: log}
}

// Execute routes one claimed task.
func (d *Dispatcher) Execute(ctx context.Context, task ClaimedTask) error {
	switch task.Type {
	case TaskExecuteNode:
		var payload StepTaskPayload
		if err := json.Unmarshal(task.Payload, &payload); err != nil {
			// A corrupt payload can never succeed; do not retry it.
			d.log.Error("corrupt task payload", "task", task.ID, "error", err)
			return nil
		}
		return d.executor.ExecuteNode(ctx, payload.RunID, payload.NodeID)
	default:
		return fmt.Errorf("no executor registered for task type %q", task.Type)
	}
}
