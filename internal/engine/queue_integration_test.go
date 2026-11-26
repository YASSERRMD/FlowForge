package engine_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/YASSERRMD/FlowForge/internal/engine"
)

// enqueueN inserts n step tasks for one run directly through the queue
// helper, each with a distinct node dedupe key.
func enqueueN(t *testing.T, f *fixture, runID string, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		if err := engine.EnqueueStep(f.ctx, f.q, runID, fmt.Sprintf("node-%d", i)); err != nil {
			t.Fatalf("enqueue %d: %v", i, err)
		}
	}
}

func TestClaimSkipsLockedRows(t *testing.T) {
	f := newFixture(t)
	queue := engine.NewQueue(f.pool)
	runID := "11111111-1111-1111-1111-111111111111"
	enqueueN(t, f, runID, 20)

	// Two workers race for the same 20 tasks with generous leases.
	const workers = 2
	claimed := make([][]engine.ClaimedTask, workers)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			tasks, err := queue.Claim(f.ctx, fmt.Sprintf("worker-%d", w), 20, 60)
			if err != nil {
				t.Errorf("claim worker-%d: %v", w, err)
				return
			}
			claimed[w] = tasks
		}(w)
	}
	wg.Wait()

	// Every task is claimed exactly once across both workers.
	seen := map[string]string{}
	total := 0
	for w := 0; w < workers; w++ {
		for _, task := range claimed[w] {
			total++
			if prev, dup := seen[task.ID]; dup {
				t.Fatalf("task %s claimed by both %s and worker-%d", task.ID, prev, w)
			}
			seen[task.ID] = fmt.Sprintf("worker-%d", w)
		}
	}
	if total != 20 {
		t.Fatalf("expected 20 claimed tasks, got %d", total)
	}
}

func TestClaimRespectsLeaseExpiry(t *testing.T) {
	f := newFixture(t)
	queue := engine.NewQueue(f.pool)
	runID := "22222222-2222-2222-2222-222222222222"
	enqueueN(t, f, runID, 1)

	first, err := queue.Claim(f.ctx, "worker-a", 1, 60)
	if err != nil {
		t.Fatalf("first claim: %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("first claim got %d tasks", len(first))
	}

	// A live lease blocks other claimants.
	second, err := queue.Claim(f.ctx, "worker-b", 1, 60)
	if err != nil {
		t.Fatalf("second claim: %v", err)
	}
	if len(second) != 0 {
		t.Fatalf("live lease should block claims, got %d", len(second))
	}

	// Expire the lease manually (simulating a crashed worker).
	if _, err := f.pool.Exec(f.ctx,
		`UPDATE scheduled_tasks SET lease_expires_at = now() - interval '1 second'`); err != nil {
		t.Fatal(err)
	}
	third, err := queue.Claim(f.ctx, "worker-b", 1, 60)
	if err != nil {
		t.Fatalf("third claim: %v", err)
	}
	if len(third) != 1 || third[0].ID != first[0].ID {
		t.Fatalf("expired lease should be reclaimable, got %+v", third)
	}
	if third[0].Attempts != 2 {
		t.Fatalf("reclaim should count as an attempt, got %d", third[0].Attempts)
	}
}

func TestEnqueueStepDeduplicates(t *testing.T) {
	f := newFixture(t)
	runID := "33333333-3333-3333-3333-333333333333"
	for i := 0; i < 3; i++ {
		if err := engine.EnqueueStep(f.ctx, f.q, runID, "node-x"); err != nil {
			t.Fatalf("enqueue %d: %v", i, err)
		}
	}
	var count int
	if err := f.sqlDB.QueryRow(
		`SELECT count(*) FROM scheduled_tasks WHERE task_type = 'execute_node'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("duplicate enqueues must collapse to one row, got %d", count)
	}
}

func TestFailReturnsToPendingThenDies(t *testing.T) {
	f := newFixture(t)
	queue := engine.NewQueue(f.pool)
	runID := "44444444-4444-4444-4444-444444444444"
	enqueueN(t, f, runID, 1)

	claimed, err := queue.Claim(f.ctx, "worker-a", 1, 60)
	if err != nil || len(claimed) != 1 {
		t.Fatalf("claim: %v (%d)", err, len(claimed))
	}
	if err := queue.Fail(f.ctx, claimed[0].ID, "worker-a", "boom", 0); err != nil {
		t.Fatalf("fail: %v", err)
	}
	var status string
	if err := f.sqlDB.QueryRow(
		`SELECT status FROM scheduled_tasks`).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "pending" {
		t.Fatalf("failed task should return to pending, got %q", status)
	}

	// Exhaust the attempt budget: the task goes dead.
	if _, err := f.pool.Exec(f.ctx,
		`UPDATE scheduled_tasks SET attempts = max_attempts`); err != nil {
		t.Fatal(err)
	}
	claimed, err = queue.Claim(f.ctx, "worker-a", 1, 60)
	if err != nil || len(claimed) != 1 {
		t.Fatalf("reclaim: %v (%d)", err, len(claimed))
	}
	if err := queue.Fail(f.ctx, claimed[0].ID, "worker-a", "boom again", 0); err != nil {
		t.Fatalf("fail: %v", err)
	}
	if err := f.sqlDB.QueryRow(
		`SELECT status FROM scheduled_tasks`).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "dead" {
		t.Fatalf("exhausted task should be dead, got %q", status)
	}
}

func TestRenewExtendsLease(t *testing.T) {
	f := newFixture(t)
	queue := engine.NewQueue(f.pool)
	runID := "55555555-5555-5555-5555-555555555555"
	enqueueN(t, f, runID, 1)

	claimed, err := queue.Claim(f.ctx, "worker-a", 1, 30)
	if err != nil || len(claimed) != 1 {
		t.Fatalf("claim: %v (%d)", err, len(claimed))
	}
	var before time.Time
	if err := f.sqlDB.QueryRow(
		`SELECT lease_expires_at FROM scheduled_tasks`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1100 * time.Millisecond)
	if err := queue.Renew(f.ctx, claimed[0].ID, "worker-a", 30); err != nil {
		t.Fatalf("renew: %v", err)
	}
	var after time.Time
	if err := f.sqlDB.QueryRow(
		`SELECT lease_expires_at FROM scheduled_tasks`).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if !after.After(before) {
		t.Fatalf("renew did not extend lease: %v -> %v", before, after)
	}
}

func TestCompleteRequiresLeaseHolder(t *testing.T) {
	f := newFixture(t)
	queue := engine.NewQueue(f.pool)
	runID := "66666666-6666-6666-6666-666666666666"
	enqueueN(t, f, runID, 1)

	claimed, err := queue.Claim(f.ctx, "worker-a", 1, 60)
	if err != nil || len(claimed) != 1 {
		t.Fatalf("claim: %v (%d)", err, len(claimed))
	}
	// A different worker cannot complete the lease.
	if err := queue.Complete(f.ctx, claimed[0].ID, "worker-b"); err != nil {
		t.Fatalf("complete by other worker: %v", err)
	}
	var status string
	if err := f.sqlDB.QueryRow(
		`SELECT status FROM scheduled_tasks`).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "leased" {
		t.Fatalf("foreign complete must not change status, got %q", status)
	}
	if err := queue.Complete(f.ctx, claimed[0].ID, "worker-a"); err != nil {
		t.Fatalf("complete: %v", err)
	}
	if err := f.sqlDB.QueryRow(
		`SELECT status FROM scheduled_tasks`).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "done" {
		t.Fatalf("owner complete should finish the task, got %q", status)
	}
}
