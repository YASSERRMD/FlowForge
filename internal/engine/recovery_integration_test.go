package engine_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/YASSERRMD/FlowForge/internal/engine"
	"github.com/YASSERRMD/FlowForge/internal/testutil"
)

func unmarshalTask(t *testing.T, raw json.RawMessage, dst any) {
	t.Helper()
	if err := json.Unmarshal(raw, dst); err != nil {
		t.Fatalf("unmarshal task payload: %v", err)
	}
}

// invoiceLikeGraph is the phase-08 gate shape: a linear invoice flow —
// fetch, approve, notify — executed with deterministic mock actions.
const invoiceLikeGraph = `{"schemaVersion":"1","nodes":[` +
	`{"id":"start-1","kind":"start","label":"Start","position":{"x":0,"y":0}},` +
	`{"id":"fetch","kind":"action","label":"Fetch invoice","position":{"x":0,"y":120},` +
	`"config":{"actionType":"mock.echo","params":{"invoiceId":"inv-100","amount":420}}},` +
	`{"id":"approve","kind":"action","label":"Approve","position":{"x":0,"y":240},` +
	`"config":{"actionType":"mock.echo","params":{"decision":"approved"}}},` +
	`{"id":"notify","kind":"action","label":"Notify finance","position":{"x":0,"y":360},` +
	`"config":{"actionType":"mock.echo","params":{"to":"finance@example.com"}}},` +
	`{"id":"end-1","kind":"end","label":"End","position":{"x":0,"y":480}}],` +
	`"edges":[` +
	`{"id":"e1","source":"start-1","target":"fetch"},` +
	`{"id":"e2","source":"fetch","target":"approve"},` +
	`{"id":"e3","source":"approve","target":"notify"},` +
	`{"id":"e4","source":"notify","target":"end-1"}]}`

// TestInvoiceLikeRunViaWorkerAndQueue is the phase gate: a full sequential
// run driven by the real worker → queue → dispatcher → executor path.
func TestInvoiceLikeRunViaWorkerAndQueue(t *testing.T) {
	f := newFixture(t)
	detail := f.createPublished("Invoice-like", invoiceLikeGraph)
	created, err := f.svc.CreateRun(f.ctx, f.ownerID, detail.ID,
		jsonRaw(`{"invoice_id":"inv-100","approval_threshold":500}`), "")
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	runID := uuidString(t, created.Run.ID.Bytes[:])

	worker := engine.NewWorker(engine.WorkerConfig{
		ID:              "gate-worker",
		Concurrency:     2,
		PollInterval:    15 * time.Millisecond,
		LeaseSeconds:    30,
		ShutdownTimeout: 2 * time.Second,
	}, engine.NewQueue(f.pool), engine.NewDispatcher(f.pool, testLog()), testLog())
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- worker.Run(ctx) }()

	waitFor(t, "run to succeed", func() bool {
		var status string
		if err := f.sqlDB.QueryRow(
			`SELECT status FROM workflow_runs WHERE id = $1`, runID).Scan(&status); err != nil {
			return false
		}
		return status == "succeeded"
	})
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("worker: %v", err)
	}

	// Exactly one succeeded attempt per node, in causal order.
	rows, err := f.sqlDB.Query(
		`SELECT node_id, status FROM step_attempts WHERE run_id = $1 ORDER BY started_at`, runID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var order []string
	for rows.Next() {
		var n, st string
		if err := rows.Scan(&n, &st); err != nil {
			t.Fatal(err)
		}
		if st != "succeeded" {
			t.Fatalf("node %s status = %q", n, st)
		}
		order = append(order, n)
	}
	want := []string{"start-1", "fetch", "approve", "notify", "end-1"}
	if len(order) != len(want) {
		t.Fatalf("execution order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("execution order = %v, want %v", order, want)
		}
	}

	// No queue residue.
	var residue int
	if err := f.sqlDB.QueryRow(
		`SELECT count(*) FROM scheduled_tasks WHERE status IN ('pending','leased')`).Scan(&residue); err != nil {
		t.Fatal(err)
	}
	if residue != 0 {
		t.Fatalf("%d tasks left over", residue)
	}
}

// TestWorkerRestartRecoversRun simulates a crash: worker A claims the start
// task and dies without executing; its lease expires; worker B reclaims and
// the run completes with exactly one attempt per node.
func TestWorkerRestartRecoversRun(t *testing.T) {
	f := newFixture(t)
	detail := f.createPublished("Restart", sequentialGraph)
	created, err := f.svc.CreateRun(f.ctx, f.ownerID, detail.ID, nil, "")
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	runID := uuidString(t, created.Run.ID.Bytes[:])

	queue := engine.NewQueue(f.pool)

	// Worker A claims and "crashes" (never executes, never completes).
	claimed, err := queue.Claim(f.ctx, "worker-a", 1, 1) // 1-second lease
	if err != nil || len(claimed) != 1 {
		t.Fatalf("worker A claim: %v (%d)", err, len(claimed))
	}
	// Worker B cannot steal the live lease.
	if stolen, _ := queue.Claim(f.ctx, "worker-b", 1, 30); len(stolen) != 0 {
		t.Fatal("worker B claimed a live lease")
	}

	// Lease expires (A is gone).
	time.Sleep(1100 * time.Millisecond)
	reclaimed, err := queue.Claim(f.ctx, "worker-b", 1, 30)
	if err != nil || len(reclaimed) != 1 {
		t.Fatalf("worker B reclaim: %v (%d)", err, len(reclaimed))
	}
	if reclaimed[0].ID != claimed[0].ID {
		t.Fatal("worker B reclaimed a different task")
	}

	// B executes and completes; the run finishes through the drain.
	var payload engine.StepTaskPayload
	unmarshalTask(t, reclaimed[0].Payload, &payload)
	ex := engine.NewExecutor(f.pool)
	if err := ex.ExecuteNode(f.ctx, payload.RunID, payload.NodeID); err != nil {
		t.Fatalf("execute reclaimed: %v", err)
	}
	if err := queue.Complete(f.ctx, reclaimed[0].ID, "worker-b"); err != nil {
		t.Fatalf("complete: %v", err)
	}
	executeDrain(t, f, runID)

	var status string
	if err := f.sqlDB.QueryRow(
		`SELECT status FROM workflow_runs WHERE id = $1`, runID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "succeeded" {
		t.Fatalf("run status after restart = %q", status)
	}

	// Exactly one attempt per node: the crash left no partial execution.
	var attempts int
	if err := f.sqlDB.QueryRow(
		`SELECT count(*) FROM step_attempts WHERE run_id = $1`, runID).Scan(&attempts); err != nil {
		t.Fatal(err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts (one per node), got %d", attempts)
	}
}

// TestRunStatePersistsAcrossReconnection proves durable state survives
// losing every connection: a fresh pool sees the same run.
func TestRunStatePersistsAcrossReconnection(t *testing.T) {
	f := newFixture(t)
	detail := f.createPublished("Durable", sequentialGraph)
	created, err := f.svc.CreateRun(f.ctx, f.ownerID, detail.ID, nil, "")
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	runID := uuidString(t, created.Run.ID.Bytes[:])
	executeDrain(t, f, runID)

	// New pool, new service — same database, same truth.
	pool2 := testutil.OpenPool(t, f.dsn)
	svc2 := engine.NewService(pool2)
	d, err := svc2.GetRunDetail(f.ctx, f.ownerID, runID)
	if err != nil {
		t.Fatalf("reconnected read: %v", err)
	}
	if d.Status != "succeeded" || len(d.Nodes) != 3 {
		t.Fatalf("reconnected view: status=%q nodes=%d", d.Status, len(d.Nodes))
	}
	pool2.Close()
}
