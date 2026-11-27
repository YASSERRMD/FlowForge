package engine_test

import (
	"encoding/json"
	"testing"
)

// cascadeGraph has a two-node chain on the untaken branch so skip propagation
// must be transitive, and a separate end node per branch.
const cascadeGraph = `{"schemaVersion":"1","nodes":[` +
	`{"id":"start-1","kind":"start","label":"Start"},` +
	`{"id":"condition-1","kind":"condition","label":"Go?","config":{"expression":"$trigger.go == true"}},` +
	`{"id":"action-true","kind":"action","label":"Yes","config":{"actionType":"mock.echo","params":{"path":"yes"}}},` +
	`{"id":"action-b","kind":"action","label":"No 1","config":{"actionType":"mock.echo","params":{"path":"no1"}}},` +
	`{"id":"action-c","kind":"action","label":"No 2","config":{"actionType":"mock.echo","params":{"path":"no2"}}},` +
	`{"id":"end-1","kind":"end","label":"End yes"},` +
	`{"id":"end-2","kind":"end","label":"End no"}],` +
	`"edges":[` +
	`{"id":"e1","source":"start-1","target":"condition-1"},` +
	`{"id":"e2","source":"condition-1","sourceHandle":"true","target":"action-true"},` +
	`{"id":"e3","source":"condition-1","sourceHandle":"false","target":"action-b"},` +
	`{"id":"e4","source":"action-b","target":"action-c"},` +
	`{"id":"e5","source":"action-c","target":"end-2"},` +
	`{"id":"e6","source":"action-true","target":"end-1"}]}`

func TestSkipPropagationIsTransitive(t *testing.T) {
	f := newFixture(t)
	detail := f.createPublished("Cascade skip", cascadeGraph)
	created, err := f.svc.CreateRun(f.ctx, f.ownerID, detail.ID,
		json.RawMessage(`{"go":true}`), "")
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	runID := uuidString(t, created.Run.ID.Bytes[:])
	executeDrain(t, f, runID)

	for _, node := range []string{"action-b", "action-c", "end-2"} {
		if got := nodeStatus(t, f, runID, node); got != "skipped" {
			t.Fatalf("%s = %q, want skipped", node, got)
		}
	}
	for _, node := range []string{"action-true", "end-1"} {
		if got := nodeStatus(t, f, runID, node); got != "succeeded" {
			t.Fatalf("%s = %q, want succeeded", node, got)
		}
	}

	var runStatus string
	if err := f.sqlDB.QueryRow(`SELECT status FROM workflow_runs WHERE id = $1`, runID).Scan(&runStatus); err != nil {
		t.Fatal(err)
	}
	if runStatus != "succeeded" {
		t.Fatalf("run = %q, want succeeded", runStatus)
	}

	// Skipped attempts are terminal and carry a reason, never a running state.
	var running int
	if err := f.sqlDB.QueryRow(
		`SELECT count(*) FROM step_attempts WHERE run_id = $1 AND status = 'running'`, runID).
		Scan(&running); err != nil {
		t.Fatal(err)
	}
	if running != 0 {
		t.Fatalf("%d attempts left running", running)
	}
}
