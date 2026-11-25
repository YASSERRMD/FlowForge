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
	lastCaller   string
	lastWorkflow string
	lastInput    json.RawMessage
	lastKey      string
	err          error
	replayed     bool
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
