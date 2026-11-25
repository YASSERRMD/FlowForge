package engine_test

import (
	"encoding/json"
	"testing"

	"github.com/YASSERRMD/FlowForge/internal/engine"
)

func jsonRaw(s string) json.RawMessage { return json.RawMessage(s) }

// jsonHas reports whether the JSON object has key == want.
func jsonHas(t *testing.T, doc, key, want string) bool {
	t.Helper()
	var obj map[string]any
	if err := json.Unmarshal([]byte(doc), &obj); err != nil {
		t.Fatalf("parse json: %v", err)
	}
	return obj[key] == want
}

// executeDrain runs queued execute_node tasks through the executor until
// the queue is empty (sequential graphs chain tasks one at a time).
func executeDrain(t *testing.T, f *fixture, runID string) {
	t.Helper()
	ex := engine.NewExecutor(f.pool)
	for i := 0; i < 50; i++ {
		var nodeID string
		err := f.sqlDB.QueryRow(
			`SELECT dedupe_key FROM scheduled_tasks
			 WHERE status = 'pending' AND task_type = 'execute_node'
			 ORDER BY created_at LIMIT 1`).Scan(&nodeID)
		if err != nil {
			return // queue drained
		}
		// dedupe key: run:<runID>:node:<nodeID>
		nodeID = nodeID[len("run:"+runID+":node:"):]
		if err := ex.ExecuteNode(f.ctx, runID, nodeID); err != nil {
			t.Fatalf("execute node %s: %v", nodeID, err)
		}
		if _, err := f.pool.Exec(f.ctx,
			`UPDATE scheduled_tasks SET status = 'done'
			 WHERE dedupe_key = $1`, "run:"+runID+":node:"+nodeID); err != nil {
			t.Fatal(err)
		}
	}
}

func TestStartNodeExecutionPassesContext(t *testing.T) {
	f := newFixture(t)
	detail := f.createPublished("Start flow", sequentialGraph)

	created, err := f.svc.CreateRun(f.ctx, f.ownerID, detail.ID,
		jsonRaw(`{"invoice_id":"inv-7","approval_threshold":500}`), "")
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	runID := uuidString(t, created.Run.ID.Bytes[:])

	ex := engine.NewExecutor(f.pool)
	if err := ex.ExecuteNode(f.ctx, runID, "start-1"); err != nil {
		t.Fatalf("execute start: %v", err)
	}

	// Run moved to running.
	var status string
	if err := f.sqlDB.QueryRow(
		`SELECT status FROM workflow_runs WHERE id = $1`, runID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "running" {
		t.Fatalf("run status = %q, want running", status)
	}

	// The start attempt succeeded with the run input as its output.
	var stepStatus, output string
	if err := f.sqlDB.QueryRow(
		`SELECT status, output_json::text FROM step_attempts
		 WHERE run_id = $1 AND node_id = 'start-1'`, runID).Scan(&stepStatus, &output); err != nil {
		t.Fatal(err)
	}
	if stepStatus != "succeeded" {
		t.Fatalf("start step status = %q", stepStatus)
	}
	if !jsonHas(t, output, "invoice_id", "inv-7") {
		t.Fatalf("start output lost run input: %s", output)
	}

	// The successor was dispatched.
	var pending int
	if err := f.sqlDB.QueryRow(
		`SELECT count(*) FROM scheduled_tasks
		 WHERE status = 'pending' AND dedupe_key = $1`,
		"run:"+runID+":node:action-1").Scan(&pending); err != nil {
		t.Fatal(err)
	}
	if pending != 1 {
		t.Fatalf("expected action-1 dispatch, got %d pending", pending)
	}

	// Events: run.started, step.started, step.succeeded in order.
	var events []string
	rows, err := f.sqlDB.Query(
		`SELECT event_type FROM run_events WHERE run_id = $1 ORDER BY sequence`, runID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var et string
		if err := rows.Scan(&et); err != nil {
			t.Fatal(err)
		}
		events = append(events, et)
	}
	want := []string{"run.created", "run.started", "step.started", "step.succeeded"}
	if len(events) != len(want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
	for i := range want {
		if events[i] != want[i] {
			t.Fatalf("event %d = %q, want %q (all: %v)", i, events[i], want[i], events)
		}
	}
}

func TestExecuteNodeOnTerminalRunIsNoop(t *testing.T) {
	f := newFixture(t)
	detail := f.createPublished("Terminal", sequentialGraph)
	created, err := f.svc.CreateRun(f.ctx, f.ownerID, detail.ID, nil, "")
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	runID := uuidString(t, created.Run.ID.Bytes[:])
	if _, err := f.pool.Exec(f.ctx,
		`UPDATE workflow_runs SET status = 'succeeded', finished_at = now() WHERE id = $1`, runID); err != nil {
		t.Fatal(err)
	}

	ex := engine.NewExecutor(f.pool)
	if err := ex.ExecuteNode(f.ctx, runID, "start-1"); err != nil {
		t.Fatalf("terminal run execute must be a no-op, got %v", err)
	}
	var attempts int
	if err := f.sqlDB.QueryRow(
		`SELECT count(*) FROM step_attempts WHERE run_id = $1`, runID).Scan(&attempts); err != nil {
		t.Fatal(err)
	}
	if attempts != 0 {
		t.Fatalf("terminal run gained %d attempts", attempts)
	}
}

func TestExecuteMissingNodeFailsRun(t *testing.T) {
	f := newFixture(t)
	detail := f.createPublished("Missing node", sequentialGraph)
	created, err := f.svc.CreateRun(f.ctx, f.ownerID, detail.ID, nil, "")
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	runID := uuidString(t, created.Run.ID.Bytes[:])

	ex := engine.NewExecutor(f.pool)
	if err := ex.ExecuteNode(f.ctx, runID, "ghost-node"); err != nil {
		t.Fatalf("missing node should fail the run, not the task: %v", err)
	}
	var status string
	if err := f.sqlDB.QueryRow(
		`SELECT status FROM workflow_runs WHERE id = $1`, runID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "failed" {
		t.Fatalf("run status = %q, want failed", status)
	}
}
