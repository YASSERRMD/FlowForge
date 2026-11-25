package engine_test

import (
	"context"
	"testing"

	"github.com/YASSERRMD/FlowForge/internal/engine"
)

// TestEventSequencesAreContiguous proves the per-run sequence is a gapless
// total order 1..N after a full sequential run.
func TestEventSequencesAreContiguous(t *testing.T) {
	f := newFixture(t)
	detail := f.createPublished("Sequences", sequentialGraph)
	created, err := f.svc.CreateRun(f.ctx, f.ownerID, detail.ID, nil, "")
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	runID := uuidString(t, created.Run.ID.Bytes[:])
	executeDrain(t, f, runID)

	rows, err := f.sqlDB.Query(
		`SELECT sequence FROM run_events WHERE run_id = $1 ORDER BY sequence`, runID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	next := int64(1)
	count := 0
	for rows.Next() {
		var seq int64
		if err := rows.Scan(&seq); err != nil {
			t.Fatal(err)
		}
		if seq != next {
			t.Fatalf("sequence gap: expected %d, got %d", next, seq)
		}
		next++
		count++
	}
	if count == 0 {
		t.Fatal("no events recorded")
	}
}

// TestEveryEventHasOutboxMirror proves events and outbox messages commit in
// the same transaction: after a full run the two streams carry exactly the
// same event types in the same order.
func TestEveryEventHasOutboxMirror(t *testing.T) {
	f := newFixture(t)
	detail := f.createPublished("Outbox mirror", sequentialGraph)
	created, err := f.svc.CreateRun(f.ctx, f.ownerID, detail.ID, nil, "")
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	runID := uuidString(t, created.Run.ID.Bytes[:])
	executeDrain(t, f, runID)

	var events, mirrored int
	if err := f.sqlDB.QueryRow(
		`SELECT count(*) FROM run_events WHERE run_id = $1`, runID).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if err := f.sqlDB.QueryRow(
		`SELECT count(*) FROM outbox_messages
		 WHERE topic = 'run.events' AND payload->>'runId' = $1`, runID).Scan(&mirrored); err != nil {
		t.Fatal(err)
	}
	if events == 0 || events != mirrored {
		t.Fatalf("events=%d mirrored=%d — outbox drifted from event stream", events, mirrored)
	}

	// Order and type agreement.
	eventRows, err := f.sqlDB.Query(
		`SELECT event_type FROM run_events WHERE run_id = $1 ORDER BY sequence`, runID)
	if err != nil {
		t.Fatal(err)
	}
	defer eventRows.Close()
	outboxRows, err := f.sqlDB.Query(
		`SELECT payload->>'type' FROM outbox_messages
		 WHERE topic = 'run.events' AND payload->>'runId' = $1 ORDER BY id`, runID)
	if err != nil {
		t.Fatal(err)
	}
	defer outboxRows.Close()
	for eventRows.Next() {
		var et string
		if err := eventRows.Scan(&et); err != nil {
			t.Fatal(err)
		}
		if !outboxRows.Next() {
			t.Fatalf("outbox shorter than event stream at %q", et)
		}
		var ot string
		if err := outboxRows.Scan(&ot); err != nil {
			t.Fatal(err)
		}
		if et != ot {
			t.Fatalf("event/outbox mismatch: %q vs %q", et, ot)
		}
	}
}

// TestFailedExecutionLeavesNoPartialEvents proves state change and events
// are atomic: a transaction that cannot commit leaves no trace.
func TestFailedExecutionLeavesNoPartialEvents(t *testing.T) {
	f := newFixture(t)
	detail := f.createPublished("Atomic", sequentialGraph)
	created, err := f.svc.CreateRun(f.ctx, f.ownerID, detail.ID, nil, "")
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	runID := uuidString(t, created.Run.ID.Bytes[:])

	// Cancel the context before execution: the transaction cannot proceed.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ex := engine.NewExecutor(f.pool)
	err = ex.ExecuteNode(ctx, runID, "start-1")
	if err == nil {
		t.Fatal("expected canceled execution to error")
	}

	var attempts, events int
	if err := f.sqlDB.QueryRow(
		`SELECT count(*) FROM step_attempts WHERE run_id = $1`, runID).Scan(&attempts); err != nil {
		t.Fatal(err)
	}
	if err := f.sqlDB.QueryRow(
		`SELECT count(*) FROM run_events WHERE run_id = $1 AND event_type != 'run.created'`,
		runID).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if attempts != 0 || events != 0 {
		t.Fatalf("canceled execution left traces: attempts=%d events=%d", attempts, events)
	}

	// The run is still queued and executable afterwards.
	if err := ex.ExecuteNode(f.ctx, runID, "start-1"); err != nil {
		t.Fatalf("execute after canceled attempt: %v", err)
	}
	var status string
	if err := f.sqlDB.QueryRow(
		`SELECT status FROM workflow_runs WHERE id = $1`, runID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "running" {
		t.Fatalf("run status after recovery = %q", status)
	}
}
