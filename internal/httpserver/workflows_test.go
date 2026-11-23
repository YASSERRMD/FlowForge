package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/YASSERRMD/FlowForge/internal/workflowstore"
)

// fakeStore implements WorkflowStore in memory.
type fakeStore struct {
	mu        sync.Mutex
	nextID    int
	workflows map[string]*storedWorkflow
}

type storedWorkflow struct {
	detail workflowstore.WorkflowDetail
	owner  string
}

func newFakeStore() *fakeStore {
	return &fakeStore{nextID: 1, workflows: map[string]*storedWorkflow{}}
}

func (f *fakeStore) EnsureUser(_ context.Context, email, _ string) (string, error) {
	return "user-" + email, nil
}

func (f *fakeStore) CreateWorkflow(_ context.Context, ownerID, name, description string, graphJSON json.RawMessage, hash []byte) (*workflowstore.WorkflowDetail, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := fmt.Sprintf("wf-%d", f.nextID)
	f.nextID++
	detail := workflowstore.WorkflowDetail{
		WorkflowSummary: workflowstore.WorkflowSummary{
			ID: id, Name: name, Description: description, Status: "draft", RowVersion: 1,
			UpdatedAt: "2026-01-01T00:00:00Z",
		},
		Draft: &workflowstore.DraftState{
			VersionID: "ver-" + id, VersionNumber: 1, RowVersion: 1,
			ContentHash: fmt.Sprintf("%x", hash), Graph: graphJSON,
		},
	}
	f.workflows[id] = &storedWorkflow{detail: detail, owner: ownerID}
	return f.workflows[id].clone(), nil
}

func (s *storedWorkflow) clone() *workflowstore.WorkflowDetail {
	c := s.detail
	if s.detail.Draft != nil {
		d := *s.detail.Draft
		c.Draft = &d
	}
	return &c
}

func (f *fakeStore) ListWorkflows(_ context.Context, ownerID string, limit, offset int32) ([]workflowstore.WorkflowSummary, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []workflowstore.WorkflowSummary
	for _, w := range f.workflows {
		if w.owner == ownerID {
			out = append(out, w.detail.WorkflowSummary)
		}
	}
	if int(offset) >= len(out) {
		return nil, nil
	}
	out = out[offset:]
	if int(limit) < len(out) {
		out = out[:limit]
	}
	return out, nil
}

func (f *fakeStore) GetWorkflow(_ context.Context, ownerID, id string) (*workflowstore.WorkflowDetail, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w, ok := f.workflows[id]
	if !ok || w.owner != ownerID {
		return nil, workflowstore.ErrNotFound
	}
	return w.clone(), nil
}

func (f *fakeStore) UpdateWorkflowMeta(_ context.Context, ownerID, id, name, description string) (*workflowstore.WorkflowSummary, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w, ok := f.workflows[id]
	if !ok || w.owner != ownerID {
		return nil, workflowstore.ErrNotFound
	}
	w.detail.Name = name
	w.detail.Description = description
	summary := w.detail.WorkflowSummary
	return &summary, nil
}

func (f *fakeStore) ArchiveWorkflow(_ context.Context, ownerID, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	w, ok := f.workflows[id]
	if !ok || w.owner != ownerID {
		return workflowstore.ErrNotFound
	}
	delete(f.workflows, id)
	return nil
}

func (f *fakeStore) SaveDraft(_ context.Context, ownerID, workflowID string, graphJSON json.RawMessage, hash []byte, expectedRowVersion int32) (*workflowstore.DraftState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w, ok := f.workflows[workflowID]
	if !ok || w.owner != ownerID {
		return nil, workflowstore.ErrNotFound
	}
	if w.detail.Draft.RowVersion != expectedRowVersion {
		return nil, workflowstore.ErrConflict
	}
	w.detail.Draft.Graph = graphJSON
	w.detail.Draft.ContentHash = fmt.Sprintf("%x", hash)
	w.detail.Draft.RowVersion++
	draft := *w.detail.Draft
	return &draft, nil
}

func workflowServer(t *testing.T) (http.Handler, *fakeStore) {
	t.Helper()
	cfg := testConfig()
	cfg.DevAuthEnabled = true
	store := newFakeStore()
	srv := New(cfg, testLogger(), WithWorkflowStore(store))
	return srv.Handler(), store
}

func doJSON(t *testing.T, h http.Handler, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(DevUserHeader, "ada")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

const validGraph = `{"schemaVersion":"1","nodes":[` +
	`{"id":"start-1","kind":"start","label":"Start"},` +
	`{"id":"end-1","kind":"end","label":"End"}],` +
	`"edges":[{"id":"e-1","source":"start-1","target":"end-1"}]}`

func createWorkflow(t *testing.T, h http.Handler) (id string, etag string) {
	t.Helper()
	rec := doJSON(t, h, http.MethodPost, "/api/v1/workflows", `{"name":"My flow"}`, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: status %d body %s", rec.Code, rec.Body)
	}
	var detail workflowstore.WorkflowDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("create: decode: %v", err)
	}
	return detail.ID, rec.Header().Get("ETag")
}

func TestWorkflowCRUDRoundTrip(t *testing.T) {
	h, _ := workflowServer(t)
	id, etag := createWorkflow(t, h)
	if etag != `"1"` {
		t.Fatalf("expected ETag \"1\", got %q", etag)
	}

	rec := doJSON(t, h, http.MethodGet, "/api/v1/workflows", "", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "My flow") {
		t.Fatalf("list: status %d body %s", rec.Code, rec.Body)
	}

	rec = doJSON(t, h, http.MethodGet, "/api/v1/workflows/"+id, "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get: status %d body %s", rec.Code, rec.Body)
	}

	rec = doJSON(t, h, http.MethodPatch, "/api/v1/workflows/"+id, `{"name":"Renamed","description":"d"}`, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Renamed") {
		t.Fatalf("patch: status %d body %s", rec.Code, rec.Body)
	}

	saveBody := `{"graph":` + validGraph + `}`
	rec = doJSON(t, h, http.MethodPut, "/api/v1/workflows/"+id+"/draft", saveBody, map[string]string{"If-Match": `"1"`})
	if rec.Code != http.StatusOK {
		t.Fatalf("save draft: status %d body %s", rec.Code, rec.Body)
	}
	if rec.Header().Get("ETag") != `"2"` {
		t.Fatalf("expected ETag \"2\" after save, got %q", rec.Header().Get("ETag"))
	}

	// Stale token must conflict.
	rec = doJSON(t, h, http.MethodPut, "/api/v1/workflows/"+id+"/draft", saveBody, map[string]string{"If-Match": `"1"`})
	if rec.Code != http.StatusConflict {
		t.Fatalf("stale save: expected 409, got %d body %s", rec.Code, rec.Body)
	}

	rec = doJSON(t, h, http.MethodDelete, "/api/v1/workflows/"+id, "", nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete: status %d", rec.Code)
	}
	rec = doJSON(t, h, http.MethodGet, "/api/v1/workflows/"+id, "", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("get after delete: expected 404, got %d", rec.Code)
	}
}

func TestWorkflowValidationErrors(t *testing.T) {
	h, _ := workflowServer(t)

	rec := doJSON(t, h, http.MethodPost, "/api/v1/workflows", `{"name":"  "}`, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("blank name: expected 400, got %d body %s", rec.Code, rec.Body)
	}

	rec = doJSON(t, h, http.MethodPost, "/api/v1/workflows", `{"name":"x","graph":{"schemaVersion":"9","nodes":[],"edges":[]}}`, nil)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad graph: expected 422, got %d body %s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "issues") {
		t.Fatalf("expected issue detail in %s", rec.Body)
	}

	id, _ := createWorkflow(t, h)
	cyclic := `{"schemaVersion":"1","nodes":[` +
		`{"id":"start-1","kind":"start","label":"S"},` +
		`{"id":"action-1","kind":"action","label":"A","config":{"actionType":"http.request"}},` +
		`{"id":"end-1","kind":"end","label":"E"}],` +
		`"edges":[{"id":"e1","source":"start-1","target":"action-1"},` +
		`{"id":"e2","source":"action-1","target":"action-1"},` +
		`{"id":"e3","source":"action-1","target":"end-1"}]}`
	rec = doJSON(t, h, http.MethodPut, "/api/v1/workflows/"+id+"/draft", `{"graph":`+cyclic+`}`, map[string]string{"If-Match": `"1"`})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("cyclic graph: expected 422, got %d body %s", rec.Code, rec.Body)
	}

	rec = doJSON(t, h, http.MethodPut, "/api/v1/workflows/"+id+"/draft", `{"graph":`+validGraph+`}`, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing If-Match: expected 400, got %d body %s", rec.Code, rec.Body)
	}
}

func TestWorkflowsRequireStore(t *testing.T) {
	cfg := testConfig()
	cfg.DevAuthEnabled = true
	srv := New(cfg, testLogger())
	rec := doJSON(t, srv.Handler(), http.MethodGet, "/api/v1/workflows", "", nil)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 without store, got %d", rec.Code)
	}
}

func TestWorkflowsRequireAuth(t *testing.T) {
	h, _ := workflowServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without identity, got %d", rec.Code)
	}
}

func TestWorkflowIsolationBetweenOwners(t *testing.T) {
	h, _ := workflowServer(t)
	id, _ := createWorkflow(t, h)

	// A different dev user must not see ada's workflow.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/"+id, nil)
	req.Header.Set(DevUserHeader, "grace")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-owner read: expected 404, got %d", rec.Code)
	}
}
