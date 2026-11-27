package engine_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/YASSERRMD/FlowForge/internal/engine"
)

const delayGraph = `{"schemaVersion":"1","nodes":[` +
	`{"id":"start-1","kind":"start","label":"Start"},` +
	`{"id":"delay-1","kind":"delay","label":"Wait","config":{"durationMs":60000}},` +
	`{"id":"end-1","kind":"end","label":"End"}],` +
	`"edges":[{"id":"e1","source":"start-1","target":"delay-1"},` +
	`{"id":"e2","source":"delay-1","target":"end-1"}]}`

func TestDelaySchedulesWakeup(t *testing.T) {
	f := newFixture(t)
	detail := f.createPublished("Delayed", delayGraph)
	created, err := f.svc.CreateRun(f.ctx, f.ownerID, detail.ID, nil, "")
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	runID := uuidString(t, created.Run.ID.Bytes[:])

	fixed := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	ex := engine.NewExecutor(f.pool).WithClock(func() time.Time { return fixed })
	if err := ex.ExecuteNode(f.ctx, runID, "start-1"); err != nil {
		t.Fatalf("start: %v", err)
	}

	// The delay node itself runs promptly; its wake time is applied to
	// successors so the run keeps one logical clock instead of drifting with
	// dispatch latency.
	if err := ex.ExecuteNode(f.ctx, runID, "delay-1"); err != nil {
		t.Fatalf("delay: %v", err)
	}
	want := fixed.Add(time.Minute)
	var endScheduled time.Time
	if err := f.sqlDB.QueryRow(
		`SELECT run_at FROM scheduled_tasks
		 WHERE dedupe_key = $1`, "run:"+runID+":node:end-1").Scan(&endScheduled); err != nil {
		t.Fatalf("end task: %v", err)
	}
	if d := endScheduled.Sub(want); d < -time.Second || d > time.Second {
		t.Fatalf("successor run_at = %s, want ~%s", endScheduled, want)
	}

	var output map[string]any
	var raw []byte
	if err := f.sqlDB.QueryRow(
		`SELECT output_json FROM step_attempts
		 WHERE run_id = $1 AND node_id = 'delay-1'`, runID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &output); err != nil {
		t.Fatal(err)
	}
	if output["durationMs"].(float64) != 60000 || output["wakeAt"] != want.Format(time.RFC3339Nano) {
		t.Fatalf("delay output = %v", output)
	}
}

func TestDelayRejectsInvalidDuration(t *testing.T) {
	f := newFixture(t)
	bad := `{"schemaVersion":"1","nodes":[` +
		`{"id":"start-1","kind":"start","label":"Start"},` +
		`{"id":"delay-1","kind":"delay","label":"Wait","config":{"durationMs":0}},` +
		`{"id":"end-1","kind":"end","label":"End"}],` +
		`"edges":[{"id":"e1","source":"start-1","target":"delay-1"},` +
		`{"id":"e2","source":"delay-1","target":"end-1"}]}`
	detail := f.createPublished("Bad delay", bad)
	created, err := f.svc.CreateRun(f.ctx, f.ownerID, detail.ID, nil, "")
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	runID := uuidString(t, created.Run.ID.Bytes[:])

	ex := engine.NewExecutor(f.pool)
	if err := ex.ExecuteNode(f.ctx, runID, "start-1"); err != nil {
		t.Fatal(err)
	}
	if err := ex.ExecuteNode(f.ctx, runID, "delay-1"); err != nil {
		t.Fatal(err)
	}
	var code string
	if err := f.sqlDB.QueryRow(
		`SELECT error_code FROM step_attempts WHERE run_id = $1 AND node_id = 'delay-1'`, runID).
		Scan(&code); err != nil {
		t.Fatal(err)
	}
	if code != "invalid_delay_config" {
		t.Fatalf("error_code = %q", code)
	}
}
