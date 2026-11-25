package engine_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/YASSERRMD/FlowForge/internal/engine"
	"github.com/YASSERRMD/FlowForge/internal/engine/actions"
)

// TestAttemptRecordsFullLifecycle proves every transition of a successful
// attempt is persisted: created pending, started with input + started_at,
// finished with output + finished_at.
func TestAttemptRecordsFullLifecycle(t *testing.T) {
	f := newFixture(t)
	detail := f.createPublished("Attempts", sequentialGraph)
	created, err := f.svc.CreateRun(f.ctx, f.ownerID, detail.ID, jsonRaw(`{"k":"v"}`), "")
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	runID := uuidString(t, created.Run.ID.Bytes[:])
	executeDrain(t, f, runID)

	rows, err := f.sqlDB.Query(
		`SELECT node_id, node_type, attempt_number, status,
		        input_json IS NOT NULL, output_json IS NOT NULL,
		        started_at IS NOT NULL, finished_at IS NOT NULL,
		        idempotency_token IS NOT NULL
		 FROM step_attempts WHERE run_id = $1 ORDER BY node_id`, runID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		count++
		var nodeID, nodeType, status string
		var attempt int
		var hasInput, hasOutput, hasStart, hasFinish, hasToken bool
		if err := rows.Scan(&nodeID, &nodeType, &attempt, &status,
			&hasInput, &hasOutput, &hasStart, &hasFinish, &hasToken); err != nil {
			t.Fatal(err)
		}
		if status != "succeeded" || attempt != 1 {
			t.Errorf("node %s: status=%q attempt=%d", nodeID, status, attempt)
		}
		if !hasInput || !hasOutput || !hasStart || !hasFinish || !hasToken {
			t.Errorf("node %s: input=%v output=%v started=%v finished=%v token=%v",
				nodeID, hasInput, hasOutput, hasStart, hasFinish, hasToken)
		}
	}
	if count != 3 {
		t.Fatalf("expected 3 attempt rows, got %d", count)
	}
}

// TestFailedAttemptRecordsError proves the failure transition persists the
// structured error fields.
func TestFailedAttemptRecordsError(t *testing.T) {
	f := newFixture(t)
	graph := `{"schemaVersion":"1","nodes":[` +
		`{"id":"start-1","kind":"start","label":"Start"},` +
		`{"id":"action-1","kind":"action","label":"Boom",` +
		`"config":{"actionType":"mock.fail","params":{"message":"kaboom"}}},` +
		`{"id":"end-1","kind":"end","label":"End"}],` +
		`"edges":[{"id":"e1","source":"start-1","target":"action-1"},` +
		`{"id":"e2","source":"action-1","target":"end-1"}]}`
	detail := f.createPublished("Failing", graph)
	created, err := f.svc.CreateRun(f.ctx, f.ownerID, detail.ID, nil, "")
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	runID := uuidString(t, created.Run.ID.Bytes[:])
	executeDrain(t, f, runID)

	var status, code, message string
	var hasFinish bool
	if err := f.sqlDB.QueryRow(
		`SELECT status, error_code, error_message, finished_at IS NOT NULL
		 FROM step_attempts WHERE run_id = $1 AND node_id = 'action-1'`, runID).
		Scan(&status, &code, &message, &hasFinish); err != nil {
		t.Fatal(err)
	}
	if status != "failed" || code != "action_failed" || message != "kaboom" || !hasFinish {
		t.Fatalf("failed attempt: status=%q code=%q msg=%q finished=%v",
			status, code, message, hasFinish)
	}
}

// TestOversizedOutputFailsSafely proves an action cannot write unbounded
// payloads: an oversized configured input fails the run as corrupt
// configuration without recording a poisoned attempt row.
func TestOversizedOutputFailsSafely(t *testing.T) {
	f := newFixture(t)
	big := strings.Repeat("x", 300*1024)
	graph := `{"schemaVersion":"1","nodes":[` +
		`{"id":"start-1","kind":"start","label":"Start"},` +
		`{"id":"action-1","kind":"action","label":"Big",` +
		`"config":{"actionType":"mock.echo","params":{"blob":"` + big + `"}}},` +
		`{"id":"end-1","kind":"end","label":"End"}],` +
		`"edges":[{"id":"e1","source":"start-1","target":"action-1"},` +
		`{"id":"e2","source":"action-1","target":"end-1"}]}`
	// The graph itself fits the publish cap (256 KiB graph limit is larger
	// than this test's graph because the blob sits in one config), so the
	// workflow publishes and the engine must bound the attempt payload.
	detail := f.createPublished("Oversized", graph)
	created, err := f.svc.CreateRun(f.ctx, f.ownerID, detail.ID, nil, "")
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	runID := uuidString(t, created.Run.ID.Bytes[:])
	executeDrain(t, f, runID)

	// No attempt row may carry the oversized payload.
	var attempts, payloadLen int
	if err := f.sqlDB.QueryRow(
		`SELECT count(*), coalesce(sum(length(coalesce(input_json::text,''))),0)
		 FROM step_attempts WHERE run_id = $1 AND node_id = 'action-1'`, runID).
		Scan(&attempts, &payloadLen); err != nil {
		t.Fatal(err)
	}
	if attempts != 0 {
		t.Fatalf("oversized input produced %d attempt rows (%d payload bytes)", attempts, payloadLen)
	}

	// The run failed with the typed error code in its event stream.
	var runStatus, errCode string
	if err := f.sqlDB.QueryRow(
		`SELECT wr.status, coalesce((SELECT payload->>'errorCode' FROM run_events
		   WHERE run_id = $1 AND event_type = 'step.failed'
		   ORDER BY sequence DESC LIMIT 1), '')
		 FROM workflow_runs wr WHERE wr.id = $1`, runID).Scan(&runStatus, &errCode); err != nil {
		t.Fatal(err)
	}
	if runStatus != "failed" || errCode != "input_too_large" {
		t.Fatalf("run=%q errorCode=%q", runStatus, errCode)
	}
}

// bloatAction returns a payload larger than the per-attempt cap from a tiny
// input, exercising the output bound.
type bloatAction struct{ size int }

func (b bloatAction) Type() string { return "mock.bloat" }
func (b bloatAction) Execute(_ context.Context, _ json.RawMessage) (json.RawMessage, error) {
	return json.RawMessage(`{"blob":"` + strings.Repeat("x", b.size) + `"}`), nil
}

func TestOversizedActionOutputFailsStep(t *testing.T) {
	f := newFixture(t)
	graph := `{"schemaVersion":"1","nodes":[` +
		`{"id":"start-1","kind":"start","label":"Start"},` +
		`{"id":"action-1","kind":"action","label":"Bloat",` +
		`"config":{"actionType":"mock.bloat","params":{}}},` +
		`{"id":"end-1","kind":"end","label":"End"}],` +
		`"edges":[{"id":"e1","source":"start-1","target":"action-1"},` +
		`{"id":"e2","source":"action-1","target":"end-1"}]}`
	detail := f.createPublished("Bloat", graph)
	created, err := f.svc.CreateRun(f.ctx, f.ownerID, detail.ID, nil, "")
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	runID := uuidString(t, created.Run.ID.Bytes[:])

	registry := actions.NewRegistry()
	registry.Register(bloatAction{size: 300 * 1024})
	ex := engine.NewExecutorWithActions(f.pool, registry)
	if err := ex.ExecuteNode(f.ctx, runID, "start-1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := ex.ExecuteNode(f.ctx, runID, "action-1"); err != nil {
		t.Fatalf("bloat step should fail durably, not error the task: %v", err)
	}

	var status, code string
	var outputLen int
	if err := f.sqlDB.QueryRow(
		`SELECT status, coalesce(error_code,''), coalesce(length(output_json::text),0)
		 FROM step_attempts WHERE run_id = $1 AND node_id = 'action-1'`, runID).
		Scan(&status, &code, &outputLen); err != nil {
		t.Fatal(err)
	}
	if status != "failed" || code != "output_too_large" {
		t.Fatalf("status=%q code=%q", status, code)
	}
	if outputLen > 0 {
		t.Fatalf("oversized output was persisted: %d bytes", outputLen)
	}
}
