package engine

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Dispatcher routes claimed tasks to the executor by task type.
type Dispatcher struct {
	pool *pgxpool.Pool
	log  *slog.Logger
}

// NewDispatcher builds a Dispatcher.
func NewDispatcher(pool *pgxpool.Pool, log *slog.Logger) *Dispatcher {
	return &Dispatcher{pool: pool, log: log}
}

// Execute routes one claimed task. Node execution lands with the executor
// in the sequential-core tasks that follow; until then execute_node fails
// loudly so the queue retries instead of silently dropping work.
func (d *Dispatcher) Execute(_ context.Context, task ClaimedTask) error {
	return fmt.Errorf("no executor registered for task type %q", task.Type)
}
