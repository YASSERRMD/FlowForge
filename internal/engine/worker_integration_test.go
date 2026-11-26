package engine_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/YASSERRMD/FlowForge/internal/engine"
)

func testLog() *slog.Logger { return slog.New(slog.NewJSONHandler(io.Discard, nil)) }

// waitFor polls cond until it holds or the deadline passes.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

type recordingHandler struct {
	mu       sync.Mutex
	executed []string
	err      error
	block    chan struct{} // when non-nil, Execute blocks until it closes
	inflight atomic_counter
	maxSeen  atomic_counter
}

type atomic_counter struct{ v int32 }

func (c *atomic_counter) inc() int32 { return atomic.AddInt32(&c.v, 1) }
func (c *atomic_counter) dec() int32 { return atomic.AddInt32(&c.v, -1) }
func (c *atomic_counter) load() int32 {
	return atomic.LoadInt32(&c.v)
}

func (h *recordingHandler) Execute(ctx context.Context, task engine.ClaimedTask) error {
	cur := h.inflight.inc()
	for {
		if m := h.maxSeen.load(); cur <= m || atomic.CompareAndSwapInt32(&h.maxSeen.v, m, cur) {
			break
		}
	}
	defer h.inflight.dec()
	if h.block != nil {
		select {
		case <-h.block:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	h.mu.Lock()
	h.executed = append(h.executed, task.ID)
	h.mu.Unlock()
	return h.err
}

func workerCfg(id string) engine.WorkerConfig {
	return engine.WorkerConfig{
		ID:              id,
		Concurrency:     4,
		PollInterval:    20 * time.Millisecond,
		LeaseSeconds:    30,
		ShutdownTimeout: 2 * time.Second,
	}
}

func TestWorkerExecutesAndCompletesTasks(t *testing.T) {
	f := newFixture(t)
	enqueueN(t, f, "77777777-7777-7777-7777-777777777777", 5)

	handler := &recordingHandler{}
	worker := engine.NewWorker(workerCfg("w-1"), engine.NewQueue(f.pool), handler, testLog())
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- worker.Run(ctx) }()

	waitFor(t, "all tasks executed", func() bool {
		handler.mu.Lock()
		defer handler.mu.Unlock()
		return len(handler.executed) == 5
	})
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("run: %v", err)
	}

	var pending int
	if err := f.sqlDB.QueryRow(
		`SELECT count(*) FROM scheduled_tasks WHERE status != 'done'`).Scan(&pending); err != nil {
		t.Fatal(err)
	}
	if pending != 0 {
		t.Fatalf("expected all tasks done, %d not done", pending)
	}
}

func TestWorkerConcurrencyLimit(t *testing.T) {
	f := newFixture(t)
	enqueueN(t, f, "88888888-8888-8888-8888-888888888888", 8)

	block := make(chan struct{})
	handler := &recordingHandler{block: block}
	cfg := workerCfg("w-1")
	cfg.Concurrency = 3
	worker := engine.NewWorker(cfg, engine.NewQueue(f.pool), handler, testLog())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = worker.Run(ctx) }()

	waitFor(t, "concurrency saturated", func() bool { return handler.inflight.load() == 3 })
	if m := handler.maxSeen.load(); m > 3 {
		t.Fatalf("concurrency exceeded: %d", m)
	}
	close(block)
	waitFor(t, "all tasks executed", func() bool {
		handler.mu.Lock()
		defer handler.mu.Unlock()
		return len(handler.executed) == 8
	})
	if m := handler.maxSeen.load(); m > 3 {
		t.Fatalf("concurrency exceeded at some point: %d", m)
	}
}

func TestWorkerRenewsLeaseDuringLongTask(t *testing.T) {
	f := newFixture(t)
	enqueueN(t, f, "99999999-9999-9999-9999-999999999999", 1)

	release := make(chan struct{})
	handler := &recordingHandler{block: release}
	cfg := workerCfg("w-1")
	cfg.LeaseSeconds = 2
	cfg.RenewInterval = 300 * time.Millisecond
	worker := engine.NewWorker(cfg, engine.NewQueue(f.pool), handler, testLog())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = worker.Run(ctx) }()

	// While the task runs longer than the raw lease, another worker must not
	// be able to claim it.
	time.Sleep(1200 * time.Millisecond)
	other := engine.NewQueue(f.pool)
	stolen, err := other.Claim(context.Background(), "w-thief", 10, 30)
	if err != nil {
		t.Fatalf("thief claim: %v", err)
	}
	if len(stolen) != 0 {
		t.Fatalf("lease was not renewed; thief claimed %d tasks", len(stolen))
	}
	close(release)
}

func TestWorkerFailureReturnsTaskToPending(t *testing.T) {
	f := newFixture(t)
	enqueueN(t, f, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", 1)

	handler := &recordingHandler{err: errors.New("boom")}
	worker := engine.NewWorker(workerCfg("w-1"), engine.NewQueue(f.pool), handler, testLog())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = worker.Run(ctx) }()

	waitFor(t, "task failed back to pending", func() bool {
		var status, lastErr string
		if err := f.sqlDB.QueryRow(
			`SELECT status, coalesce(last_error,'') FROM scheduled_tasks`).Scan(&status, &lastErr); err != nil {
			return false
		}
		return status == "pending" && lastErr == "boom"
	})
	cancel()
}

func TestWorkerGracefulShutdown(t *testing.T) {
	f := newFixture(t)
	enqueueN(t, f, "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", 2)

	block := make(chan struct{})
	handler := &recordingHandler{block: block}
	cfg := workerCfg("w-1")
	cfg.ShutdownTimeout = 500 * time.Millisecond
	worker := engine.NewWorker(cfg, engine.NewQueue(f.pool), handler, testLog())
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- worker.Run(ctx) }()

	waitFor(t, "tasks in flight", func() bool { return handler.inflight.load() == 2 })
	start := time.Now()
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("worker did not shut down")
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("shutdown took too long: %v", elapsed)
	}
	close(block) // let the handler goroutines unwind
}
