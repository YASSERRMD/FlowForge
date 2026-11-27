package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/YASSERRMD/FlowForge/internal/db/dbgen"
	"github.com/YASSERRMD/FlowForge/internal/engine"
)

type fakeEngine struct {
	detail    *engine.RunDetail
	detailErr error

	lastCaller   string
	lastWorkflow string
	lastInput    json.RawMessage
	lastKey      string
	lastRun      string
	err          error
	replayed     bool

	runContext    *engine.RunContextView
	runContextErr error
	notifications []engine.NotificationView
	notifErr      error
}

func (f *fakeEngine) CreateRun(_ context.Context, callerID, workflowID string,
	input json.RawMessage, idempotencyKey string) (*engine.CreatedRun, error) {
	f.lastCaller, f.lastWorkflow = callerID, workflowID
	f.lastInput, f.lastKey = input, idempotencyKey
	if f.err != nil {
		return nil, f.err
	}
	var runID, wfID pgtype.UUID
	_ = runID.Scan("11111111-1111-1111-1111-111111111111")
	_ = wfID.Scan("22222222-2222-2222-2222-222222222222")
	return &engine.CreatedRun{
		Run: dbgen.WorkflowRun{
			ID:          runID,
			WorkflowID:  wfID,
			Status:      "queued",
			TriggerType: "manual",
			CreatedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
		},
		Version:      dbgen.WorkflowVersion{VersionNumber: 1},
		ReplayedFrom: f.replayed,
	}, nil
}

func (f *fakeEngine) GetRunDetail(_ context.Context, callerID, runID string) (*engine.RunDetail, error) {
	if f.detailErr != nil {
		return nil, f.detailErr
	}
	return f.detail, nil
}

func (f *fakeEngine) GetRunContext(_ context.Context, callerID, runID string) (*engine.RunContextView, error) {
	f.lastRun = runID
	if f.runContextErr != nil {
		return nil, f.runContextErr
	}
	return f.runContext, nil
}

func (f *fakeEngine) ListRunNotifications(_ context.Context, callerID, runID string) ([]engine.NotificationView, error) {
	f.lastRun = runID
	if f.notifErr != nil {
		return nil, f.notifErr
	}
	return f.notifications, nil
}

func newRunServer(t *testing.T, eng EngineService) http.Handler {
	t.Helper()
	cfg := testConfig()
	cfg.DevAuthEnabled = true
	return New(cfg, testLogger(), WithWorkflowStore(&fakeStore{}), WithEngine(eng)).Handler()
}

func TestCreateRunRoute(t *testing.T) {
	eng := &fakeEngine{}
	h := newRunServer(t, eng)

	rec := doJSON(t, h, http.MethodPost,
		"/api/v1/workflows/22222222-2222-2222-2222-222222222222/runs",
		`{"input":{"invoice_id":"inv-1"},"idempotencyKey":"k-1"}`, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if eng.lastWorkflow != "22222222-2222-2222-2222-222222222222" {
		t.Fatalf("workflow = %q", eng.lastWorkflow)
	}
	if eng.lastKey != "k-1" {
		t.Fatalf("idempotency key = %q", eng.lastKey)
	}

	var body struct {
		ID       string `json:"id"`
		Status   string `json:"status"`
		Replayed bool   `json:"replayed"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.ID != "11111111-1111-1111-1111-111111111111" || body.Status != "queued" || body.Replayed {
		t.Fatalf("unexpected body: %+v", body)
	}
}

func TestCreateRunEmptyBodyAllowed(t *testing.T) {
	eng := &fakeEngine{}
	h := newRunServer(t, eng)
	rec := doJSON(t, h, http.MethodPost,
		"/api/v1/workflows/22222222-2222-2222-2222-222222222222/runs", "", nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestCreateRunReplayReturns200(t *testing.T) {
	eng := &fakeEngine{replayed: true}
	h := newRunServer(t, eng)
	rec := doJSON(t, h, http.MethodPost,
		"/api/v1/workflows/22222222-2222-2222-2222-222222222222/runs",
		`{"idempotencyKey":"k-1"}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("replay status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestCreateRunErrorMapping(t *testing.T) {
	cases := []struct {
		name string
		err  error
		code int
	}{
		{"not found", engine.ErrWorkflowNotFound, http.StatusNotFound},
		{"no published version", engine.ErrNoPublishedVersion, http.StatusConflict},
		{"invalid input", engine.ErrInvalidInput, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			eng := &fakeEngine{err: tc.err}
			h := newRunServer(t, eng)
			rec := doJSON(t, h, http.MethodPost,
				"/api/v1/workflows/22222222-2222-2222-2222-222222222222/runs", "", nil)
			if rec.Code != tc.code {
				t.Fatalf("status = %d, want %d, body = %s", rec.Code, tc.code, rec.Body.String())
			}
		})
	}
}

func TestCreateRunRequiresEngine(t *testing.T) {
	cfg := testConfig()
	cfg.DevAuthEnabled = true
	h := New(cfg, testLogger(), WithWorkflowStore(&fakeStore{})).Handler()
	rec := doJSON(t, h, http.MethodPost,
		"/api/v1/workflows/22222222-2222-2222-2222-222222222222/runs", "", nil)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestGetRunRoute(t *testing.T) {
	eng := &fakeEngine{detail: &engine.RunDetail{
		ID: "11111111-1111-1111-1111-111111111111", Status: "succeeded",
		VersionNumber: 3, DurationMs: 12,
		Nodes: []engine.NodeState{{NodeID: "start-1", NodeType: "start", Status: "succeeded"}},
	}}
	h := newRunServer(t, eng)
	rec := doJSON(t, h, http.MethodGet,
		"/api/v1/runs/11111111-1111-1111-1111-111111111111", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		ID       string `json:"id"`
		Status   string `json:"status"`
		Duration int64  `json:"durationMs"`
		Nodes    []struct {
			NodeID string `json:"nodeId"`
			Status string `json:"status"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Status != "succeeded" || body.Duration != 12 || len(body.Nodes) != 1 {
		t.Fatalf("unexpected body: %+v", body)
	}
}

func TestGetRunNotFound(t *testing.T) {
	eng := &fakeEngine{detailErr: engine.ErrRunNotFound}
	h := newRunServer(t, eng)
	rec := doJSON(t, h, http.MethodGet,
		"/api/v1/runs/99999999-9999-9999-9999-999999999999", "", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestGetRunContextRoute(t *testing.T) {
	eng := &fakeEngine{runContext: &engine.RunContextView{
		Version: 1,
		Trigger: json.RawMessage(`{"invoice_id":"inv-1"}`),
		Steps:   map[string]json.RawMessage{"action-1": json.RawMessage(`{"valid":true}`)},
	}}
	h := newRunServer(t, eng)
	rec := doJSON(t, h, http.MethodGet,
		"/api/v1/runs/33333333-3333-3333-3333-333333333333/context", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if eng.lastRun != "33333333-3333-3333-3333-333333333333" {
		t.Fatalf("run = %q", eng.lastRun)
	}
	var body engine.RunContextView
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Version != 1 || string(body.Trigger) != `{"invoice_id":"inv-1"}` {
		t.Fatalf("body = %+v", body)
	}
	if string(body.Steps["action-1"]) != `{"valid":true}` {
		t.Fatalf("steps = %v", body.Steps)
	}
}

func TestGetRunContextNotFound(t *testing.T) {
	eng := &fakeEngine{runContextErr: engine.ErrRunNotFound}
	h := newRunServer(t, eng)
	rec := doJSON(t, h, http.MethodGet,
		"/api/v1/runs/33333333-3333-3333-3333-333333333333/context", "", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestListRunNotificationsRoute(t *testing.T) {
	eng := &fakeEngine{notifications: []engine.NotificationView{{
		ID:        "44444444-4444-4444-4444-444444444444",
		NodeID:    "notify-1",
		Channel:   "email",
		Recipient: "ada@example.com",
		Subject:   "Hello",
		Payload:   json.RawMessage(`{"sent":true}`),
	}}}
	h := newRunServer(t, eng)
	rec := doJSON(t, h, http.MethodGet,
		"/api/v1/runs/33333333-3333-3333-3333-333333333333/notifications", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Notifications []engine.NotificationView `json:"notifications"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Notifications) != 1 || body.Notifications[0].Recipient != "ada@example.com" {
		t.Fatalf("notifications = %+v", body.Notifications)
	}
}
