package engine_test

import (
	"encoding/json"
	"testing"
)

const notifyGraph = `{"schemaVersion":"1","nodes":[` +
	`{"id":"start-1","kind":"start","label":"Start"},` +
	`{"id":"notify-1","kind":"action","label":"Email","config":{"actionType":"mock.notify.email",` +
	`"params":{"to":"$trigger.email","subject":"$trigger.subject"}}},` +
	`{"id":"end-1","kind":"end","label":"End"}],` +
	`"edges":[{"id":"e1","source":"start-1","target":"notify-1"},` +
	`{"id":"e2","source":"notify-1","target":"end-1"}]}`

func TestNotificationActionPersistsRecord(t *testing.T) {
	f := newFixture(t)
	detail := f.createPublished("Notify", notifyGraph)
	created, err := f.svc.CreateRun(f.ctx, f.ownerID, detail.ID,
		json.RawMessage(`{"email":"ada@example.com","subject":"Invoice inv-7 ready"}`), "")
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	runID := uuidString(t, created.Run.ID.Bytes[:])
	executeDrain(t, f, runID)

	rows, err := f.q.ListNotificationsByRun(f.ctx, created.Run.ID)
	if err != nil {
		t.Fatalf("list notifications: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("notifications = %d, want 1", len(rows))
	}
	got := rows[0]
	if got.NodeID != "notify-1" || got.Channel != "email" {
		t.Fatalf("notification = node %q channel %q", got.NodeID, got.Channel)
	}
	if got.Recipient == nil || *got.Recipient != "ada@example.com" {
		t.Fatalf("recipient = %v", got.Recipient)
	}
	if got.Subject == nil || *got.Subject != "Invoice inv-7 ready" {
		t.Fatalf("subject = %v", got.Subject)
	}
	var payload map[string]any
	if err := json.Unmarshal(got.Payload, &payload); err != nil || payload["sent"] != true {
		t.Fatalf("payload = %s", got.Payload)
	}

	// The record is run-scoped: a second run must not see the first's rows.
	other, err := f.svc.CreateRun(f.ctx, f.ownerID, detail.ID,
		json.RawMessage(`{"email":"grace@example.com"}`), "")
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if rows, err := f.q.ListNotificationsByRun(f.ctx, other.Run.ID); err != nil || len(rows) != 0 {
		t.Fatalf("second run notifications = %d, %v", len(rows), err)
	}
}

func TestNonNotificationActionDoesNotPersist(t *testing.T) {
	f := newFixture(t)
	detail := f.createPublished("Echo", sequentialGraph)
	created, err := f.svc.CreateRun(f.ctx, f.ownerID, detail.ID, nil, "")
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	executeDrain(t, f, uuidString(t, created.Run.ID.Bytes[:]))
	rows, err := f.q.ListNotificationsByRun(f.ctx, created.Run.ID)
	if err != nil {
		t.Fatalf("list notifications: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("unexpected notifications: %d", len(rows))
	}
}
