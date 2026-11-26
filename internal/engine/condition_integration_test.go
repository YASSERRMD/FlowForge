package engine_test

import (
	"encoding/json"
	"testing"

	"github.com/YASSERRMD/FlowForge/internal/engine"
)

// conditionGraph branches on the trigger amount, then merges both branches on
// a shared node so skip propagation can be verified against a join-like path.
const conditionGraph = `{"schemaVersion":"1","nodes":[` +
	`{"id":"start-1","kind":"start","label":"Start"},` +
	`{"id":"condition-1","kind":"condition","label":"Over threshold?",` +
	`"config":{"expression":"$trigger.amount > 100"}},` +
	`{"id":"action-true","kind":"action","label":"High","config":{"actionType":"mock.echo","params":{"path":"high"}}},` +
	`{"id":"action-false","kind":"action","label":"Low","config":{"actionType":"mock.echo","params":{"path":"low"}}},` +
	`{"id":"action-merge","kind":"action","label":"Merge","config":{"actionType":"mock.echo","params":{"path":"merge"}}},` +
	`{"id":"end-1","kind":"end","label":"End"}],` +
	`"edges":[` +
	`{"id":"e1","source":"start-1","target":"condition-1"},` +
	`{"id":"e2","source":"condition-1","sourceHandle":"true","target":"action-true"},` +
	`{"id":"e3","source":"condition-1","sourceHandle":"false","target":"action-false"},` +
	`{"id":"e4","source":"action-true","target":"action-merge"},` +
	`{"id":"e5","source":"action-false","target":"action-merge"},` +
	`{"id":"e6","source":"action-merge","target":"end-1"}]}`

func nodeStatus(t *testing.T, f *fixture, runID, nodeID string) string {
	t.Helper()
	var status string
	if err := f.sqlDB.QueryRow(
		`SELECT status FROM step_attempts WHERE run_id = $1 AND node_id = $2
		 ORDER BY attempt_number DESC LIMIT 1`, runID, nodeID).Scan(&status); err != nil {
		t.Fatalf("status of %s: %v", nodeID, err)
	}
	return status
}

func TestConditionSelectsTrueBranch(t *testing.T) {
	f := newFixture(t)
	detail := f.createPublished("Branch high", conditionGraph)
	created, err := f.svc.CreateRun(f.ctx, f.ownerID, detail.ID,
		json.RawMessage(`{"amount":250}`), "")
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	runID := uuidString(t, created.Run.ID.Bytes[:])
	executeDrain(t, f, runID)

	if got := nodeStatus(t, f, runID, "action-true"); got != "succeeded" {
		t.Fatalf("true branch = %q, want succeeded", got)
	}
	if got := nodeStatus(t, f, runID, "action-false"); got != "skipped" {
		t.Fatalf("false branch = %q, want skipped", got)
	}
	if got := nodeStatus(t, f, runID, "action-merge"); got != "succeeded" {
		t.Fatalf("shared merge = %q, want succeeded (not skipped)", got)
	}

	var runStatus string
	if err := f.sqlDB.QueryRow(`SELECT status FROM workflow_runs WHERE id = $1`, runID).Scan(&runStatus); err != nil {
		t.Fatal(err)
	}
	if runStatus != "succeeded" {
		t.Fatalf("run = %q, want succeeded", runStatus)
	}

	var skips int
	if err := f.sqlDB.QueryRow(
		`SELECT count(*) FROM run_events WHERE run_id = $1 AND event_type = 'step.skipped'`, runID).
		Scan(&skips); err != nil {
		t.Fatal(err)
	}
	if skips != 1 {
		t.Fatalf("step.skipped events = %d, want 1", skips)
	}
}

func TestConditionSelectsFalseBranch(t *testing.T) {
	f := newFixture(t)
	detail := f.createPublished("Branch low", conditionGraph)
	created, err := f.svc.CreateRun(f.ctx, f.ownerID, detail.ID,
		json.RawMessage(`{"amount":10}`), "")
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	runID := uuidString(t, created.Run.ID.Bytes[:])
	executeDrain(t, f, runID)

	if got := nodeStatus(t, f, runID, "action-false"); got != "succeeded" {
		t.Fatalf("false branch = %q, want succeeded", got)
	}
	if got := nodeStatus(t, f, runID, "action-true"); got != "skipped" {
		t.Fatalf("true branch = %q, want skipped", got)
	}
	if got := nodeStatus(t, f, runID, "action-merge"); got != "succeeded" {
		t.Fatalf("shared merge = %q, want succeeded", got)
	}
}

func TestConditionEvaluationErrorFailsRun(t *testing.T) {
	f := newFixture(t)
	bad := `{"schemaVersion":"1","nodes":[` +
		`{"id":"start-1","kind":"start","label":"Start"},` +
		`{"id":"condition-1","kind":"condition","label":"Bad",` +
		`"config":{"expression":"$trigger.missing > 5"}},` +
		`{"id":"end-1","kind":"end","label":"End"}],` +
		`"edges":[{"id":"e1","source":"start-1","target":"condition-1"},` +
		`{"id":"e2","source":"condition-1","sourceHandle":"true","target":"end-1"},` +
		`{"id":"e3","source":"condition-1","sourceHandle":"false","target":"end-1"}]}`
	detail := f.createPublished("Bad expression", bad)
	created, err := f.svc.CreateRun(f.ctx, f.ownerID, detail.ID, nil, "")
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	runID := uuidString(t, created.Run.ID.Bytes[:])

	ex := engine.NewExecutor(f.pool)
	if err := ex.ExecuteNode(f.ctx, runID, "start-1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := ex.ExecuteNode(f.ctx, runID, "condition-1"); err != nil {
		t.Fatalf("condition task must complete, got %v", err)
	}
	var stepStatus, code, runStatus string
	if err := f.sqlDB.QueryRow(
		`SELECT sa.status, sa.error_code, wr.status FROM step_attempts sa
		 JOIN workflow_runs wr ON wr.id = sa.run_id
		 WHERE sa.run_id = $1 AND sa.node_id = 'condition-1'`, runID).
		Scan(&stepStatus, &code, &runStatus); err != nil {
		t.Fatal(err)
	}
	if stepStatus != "failed" || code != "expression_error" || runStatus != "failed" {
		t.Fatalf("step=%q code=%q run=%q", stepStatus, code, runStatus)
	}
}
