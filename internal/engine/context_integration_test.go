package engine_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/YASSERRMD/FlowForge/internal/engine"
)

func TestGetRunContextProjectsSucceededOutputs(t *testing.T) {
	f := newFixture(t)
	detail := f.createPublished("Context", sequentialGraph)
	created, err := f.svc.CreateRun(f.ctx, f.ownerID, detail.ID,
		json.RawMessage(`{"invoice_id":"inv-7"}`), "")
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	runID := uuidString(t, created.Run.ID.Bytes[:])
	executeDrain(t, f, runID)

	view, err := f.svc.GetRunContext(f.ctx, f.ownerID, runID)
	if err != nil {
		t.Fatalf("context: %v", err)
	}
	if view.Version != 1 {
		t.Fatalf("version = %d", view.Version)
	}
	var trigger map[string]any
	if err := json.Unmarshal(view.Trigger, &trigger); err != nil || trigger["invoice_id"] != "inv-7" {
		t.Fatalf("trigger = %s", view.Trigger)
	}
	output, ok := view.Steps["action-1"]
	if !ok {
		t.Fatalf("missing action-1 step: %v", view.Steps)
	}
	var out map[string]any
	if err := json.Unmarshal(output, &out); err != nil || out["ok"] != true {
		t.Fatalf("action-1 output = %s", output)
	}
}

func TestRunInspectionIsOwnerScoped(t *testing.T) {
	f := newFixture(t)
	detail := f.createPublished("Scoped", sequentialGraph)
	created, err := f.svc.CreateRun(f.ctx, f.ownerID, detail.ID, nil, "")
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	runID := uuidString(t, created.Run.ID.Bytes[:])

	other, err := f.store.EnsureUser(f.ctx, "dev-grace@dev.flowforge.local", "grace")
	if err != nil {
		t.Fatalf("ensure other: %v", err)
	}
	if _, err := f.svc.GetRunContext(f.ctx, other, runID); !errors.Is(err, engine.ErrRunNotFound) {
		t.Fatalf("foreign context err = %v", err)
	}
	if _, err := f.svc.ListRunNotifications(f.ctx, other, runID); !errors.Is(err, engine.ErrRunNotFound) {
		t.Fatalf("foreign notifications err = %v", err)
	}
	if _, err := f.svc.GetRunContext(f.ctx, f.ownerID, "not-a-uuid"); !errors.Is(err, engine.ErrRunNotFound) {
		t.Fatalf("bad id err = %v", err)
	}
}
