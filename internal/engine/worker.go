package engine

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

// TaskHandler executes one claimed queue task. The workflow executor
// implements it for execute_node tasks.
type TaskHandler interface {
	Execute(ctx context.Context, task ClaimedTask) error
}

// WorkerConfig tunes the worker lifecycle.
type WorkerConfig struct {
	// ID identifies this worker in task leases.
	ID string
	// Concurrency caps simultaneously executing tasks.
	Concurrency int
	// PollInterval is the delay between queue polls when idle.
	PollInterval time.Duration
	// LeaseSeconds bounds how long one claim lasts without renewal.
	LeaseSeconds int32
	// RenewInterval is how often a running task's lease is extended;
	// defaults to LeaseSeconds/3.
	RenewInterval time.Duration
	// ShutdownTimeout bounds the wait for in-flight tasks during graceful
	// shutdown; afterwards their leases simply expire.
	ShutdownTimeout time.Duration
}

// Worker is the long-running execution process: it claims durable tasks
// with bounded leases, executes them with bounded concurrency, renews
// leases while working and shuts down gracefully.
type Worker struct {
	cfg     WorkerConfig
	queue   *Queue
	handler TaskHandler
	log     *slog.Logger
}

// NewWorker builds a Worker.
func NewWorker(cfg WorkerConfig, queue *Queue, handler TaskHandler, log *slog.Logger) *Worker {
	if cfg.Concurrency < 1 {
		cfg.Concurrency = 1
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 250 * time.Millisecond
	}
	if cfg.LeaseSeconds < 1 {
		cfg.LeaseSeconds = 30
	}
	if cfg.RenewInterval <= 0 {
		cfg.RenewInterval = time.Duration(cfg.LeaseSeconds) * time.Second / 3
	}
	if cfg.ShutdownTimeout <= 0 {
		cfg.ShutdownTimeout = 10 * time.Second
	}
	if log == nil {
		log = slog.Default()
	}
	return &Worker{cfg: cfg, queue: queue, handler: handler, log: log}
}

// Run polls and executes until ctx is canceled, then waits for in-flight
// tasks up to ShutdownTimeout.
func (w *Worker) Run(ctx context.Context) error {
	sem := make(chan struct{}, w.cfg.Concurrency)
	var wg sync.WaitGroup

	poll := time.NewTimer(0)
	defer poll.Stop()
	for {
		select {
		case <-ctx.Done():
			w.waitInflight(&wg)
			return nil
		case <-poll.C:
		}

		free := int32(cap(sem) - len(sem))
		if free <= 0 {
			poll.Reset(w.cfg.PollInterval)
			continue
		}
		tasks, err := w.queue.Claim(ctx, w.cfg.ID, free, w.cfg.LeaseSeconds)
		if err != nil {
			if ctx.Err() != nil {
				continue
			}
			w.log.Warn("claim failed; backing off", "error", err)
			poll.Reset(w.cfg.PollInterval)
			continue
		}
		if len(tasks) == 0 {
			poll.Reset(w.cfg.PollInterval)
			continue
		}
		for _, task := range tasks {
			sem <- struct{}{}
			wg.Add(1)
			go func(task ClaimedTask) {
				defer wg.Done()
				defer func() { <-sem }()
				w.execute(ctx, task)
			}(task)
		}
		poll.Reset(0) // immediately check for more work
	}
}

// waitInflight blocks until running tasks finish or ShutdownTimeout passes;
// remaining leases expire on their own and another worker reclaims them.
func (w *Worker) waitInflight(wg *sync.WaitGroup) {
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(w.cfg.ShutdownTimeout):
		w.log.Warn("shutdown timeout exceeded; leases left to expire")
	}
}

// execute runs one task with lease renewal, then completes or fails it.
func (w *Worker) execute(ctx context.Context, task ClaimedTask) {
	taskCtx, stopRenew := context.WithCancel(ctx)
	defer stopRenew()
	go w.renewLoop(taskCtx, task)

	err := w.handler.Execute(taskCtx, task)
	stopRenew()

	// Completing/failing uses a fresh context: shutdown must not strand a
	// finished task in leased state.
	writeCtx, cancel := context.WithTimeout(context.Background(), w.cfg.ShutdownTimeout)
	defer cancel()
	if err == nil {
		if cerr := w.queue.Complete(writeCtx, task.ID, w.cfg.ID); cerr != nil {
			w.log.Error("complete task", "task", task.ID, "error", cerr)
		}
		return
	}
	if errors.Is(err, context.Canceled) && ctx.Err() != nil {
		w.log.Info("task interrupted by shutdown; lease will expire", "task", task.ID)
		return
	}
	w.log.Warn("task failed", "task", task.ID, "dedupe", task.Dedupe, "error", err)
	backoff := backoffSeconds(task.Attempts)
	if ferr := w.queue.Fail(writeCtx, task.ID, w.cfg.ID, err.Error(), backoff); ferr != nil {
		w.log.Error("fail task", "task", task.ID, "error", ferr)
	}
}

// renewLoop extends the task lease until the task context ends.
func (w *Worker) renewLoop(ctx context.Context, task ClaimedTask) {
	tick := time.NewTicker(w.cfg.RenewInterval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			if err := w.queue.Renew(ctx, task.ID, w.cfg.ID, w.cfg.LeaseSeconds); err != nil && ctx.Err() == nil {
				w.log.Warn("lease renewal failed", "task", task.ID, "error", err)
			}
		}
	}
}

// backoffSeconds is a capped exponential retry delay based on the number
// of attempts already consumed.
func backoffSeconds(attempts int32) int32 {
	delay := int32(1)
	for i := int32(0); i < attempts && delay < 300; i++ {
		delay *= 2
	}
	if delay > 300 {
		delay = 300
	}
	return delay
}
