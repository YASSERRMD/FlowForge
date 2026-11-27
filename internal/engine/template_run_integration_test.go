package engine_test

import (
	"encoding/json"
	"testing"

	"github.com/YASSERRMD/FlowForge/internal/engine/actions"
	"github.com/YASSERRMD/FlowForge/internal/templates"
)

func templateGraph(t *testing.T, id string) (string, *json.RawMessage) {
	t.Helper()
	tpl, ok := templates.Get(id)
	if !ok {
		t.Fatalf("template %q missing", id)
	}
	return string(tpl.Graph), &tpl.Graph
}

// TestTemplatesUseRegisteredActions guards the seed data: every action type a
// template references must exist in the deterministic catalog.
func TestTemplatesUseRegisteredActions(t *testing.T) {
	reg := actions.Default()
	for _, tpl := range templates.List() {
		var g struct {
			Nodes []struct {
				Kind   string `json:"kind"`
				Config struct {
					ActionType string `json:"actionType"`
				} `json:"config"`
			} `json:"nodes"`
		}
		if err := json.Unmarshal(tpl.Graph, &g); err != nil {
			t.Fatalf("%s: %v", tpl.ID, err)
		}
		for _, n := range g.Nodes {
			if n.Kind == "action" && !reg.Has(n.Config.ActionType) {
				t.Fatalf("%s references unregistered action %q", tpl.ID, n.Config.ActionType)
			}
		}
	}
}

func TestInvoiceApprovalTemplateAutoApprove(t *testing.T) {
	f := newFixture(t)
	graphJSON, _ := templateGraph(t, "invoice-approval")
	detail := f.createPublished("Invoice auto", graphJSON)
	created, err := f.svc.CreateRun(f.ctx, f.ownerID, detail.ID,
		json.RawMessage(`{"invoice_id":"inv-accept-1","approval_threshold":1000}`), "")
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	runID := uuidString(t, created.Run.ID.Bytes[:])
	executeDrain(t, f, runID)

	takeBranch := func(node string) string { return nodeStatus(t, f, runID, node) }
	if takeBranch("action-2") != "succeeded" || takeBranch("action-3") != "succeeded" {
		t.Fatalf("approve path: action-2=%s action-3=%s",
			takeBranch("action-2"), takeBranch("action-3"))
	}
	if takeBranch("action-4") != "skipped" || takeBranch("action-5") != "skipped" {
		t.Fatalf("review path should be skipped: action-4=%s action-5=%s",
			takeBranch("action-4"), takeBranch("action-5"))
	}
	notifications, err := f.svc.ListRunNotifications(f.ctx, f.ownerID, runID)
	if err != nil {
		t.Fatalf("notifications: %v", err)
	}
	if len(notifications) != 1 || notifications[0].Recipient != "finance@example.com" {
		t.Fatalf("notifications = %+v", notifications)
	}
	assertRunSucceeded(t, f, runID)
}

func TestInvoiceApprovalTemplateReviewBranch(t *testing.T) {
	f := newFixture(t)
	graphJSON, _ := templateGraph(t, "invoice-approval")
	detail := f.createPublished("Invoice review", graphJSON)
	created, err := f.svc.CreateRun(f.ctx, f.ownerID, detail.ID,
		json.RawMessage(`{"invoice_id":"inv-accept-2","approval_threshold":1,
			"requester_email":"ada@example.com"}`), "")
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	runID := uuidString(t, created.Run.ID.Bytes[:])
	executeDrain(t, f, runID)

	if got := nodeStatus(t, f, runID, "action-4"); got != "succeeded" {
		t.Fatalf("review = %q", got)
	}
	if got := nodeStatus(t, f, runID, "action-5"); got != "succeeded" {
		t.Fatalf("notify requester = %q", got)
	}
	if got := nodeStatus(t, f, runID, "action-2"); got != "skipped" {
		t.Fatalf("auto-approve = %q, want skipped", got)
	}
	notifications, err := f.svc.ListRunNotifications(f.ctx, f.ownerID, runID)
	if err != nil {
		t.Fatalf("notifications: %v", err)
	}
	if len(notifications) != 1 || notifications[0].Recipient != "ada@example.com" {
		t.Fatalf("notifications = %+v", notifications)
	}
	assertRunSucceeded(t, f, runID)
}

func TestIncidentEscalationTemplate(t *testing.T) {
	f := newFixture(t)
	graphJSON, _ := templateGraph(t, "incident-escalation")
	detail := f.createPublished("Incident", graphJSON)
	created, err := f.svc.CreateRun(f.ctx, f.ownerID, detail.ID,
		json.RawMessage(`{"on_call":"ada","incident_id":"inc-accept-1"}`), "")
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	runID := uuidString(t, created.Run.ID.Bytes[:])
	executeDrain(t, f, runID)

	if got := nodeStatus(t, f, runID, "action-2"); got != "skipped" {
		t.Fatalf("ack path = %q, want skipped", got)
	}
	for _, node := range []string{"action-3", "action-4", "action-5"} {
		if got := nodeStatus(t, f, runID, node); got != "succeeded" {
			t.Fatalf("%s = %q", node, got)
		}
	}
	notifications, err := f.svc.ListRunNotifications(f.ctx, f.ownerID, runID)
	if err != nil {
		t.Fatalf("notifications: %v", err)
	}
	if len(notifications) != 1 {
		t.Fatalf("notifications = %+v", notifications)
	}
	if n := notifications[0]; n.Channel != "statuspage" || n.Recipient != "inc-accept-1" {
		t.Fatalf("statuspage notification = %+v", n)
	}
	assertRunSucceeded(t, f, runID)
}

func assertRunSucceeded(t *testing.T, f *fixture, runID string) {
	t.Helper()
	var status string
	if err := f.sqlDB.QueryRow(`SELECT status FROM workflow_runs WHERE id = $1`, runID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "succeeded" {
		t.Fatalf("run = %q, want succeeded", status)
	}
}
