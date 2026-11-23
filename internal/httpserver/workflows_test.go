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
	detail       workflowstore.WorkflowDetail
	owner        string
	publishCount int
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

func (f *fakeStore) Publish(_ context.Context, ownerID, workflowID string) (*workflowstore.PublishedVersion, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w, ok := f.workflows[workflowID]
	if !ok || w.owner != ownerID {
		return nil, workflowstore.ErrNotFound
	}
	if w.detail.Draft == nil {
		return nil, workflowstore.ErrNoDraft
	}
	n := w.publishCount + 1
	w.publishCount = n
	w.detail.Draft = &workflowstore.DraftState{
		VersionID:     "ver-new",
		VersionNumber: int32(n + 1),
		RowVersion:    1,
		ContentHash:   w.detail.Draft.ContentHash,
		Graph:         w.detail.Draft.Graph,
	}
	return &workflowstore.PublishedVersion{
		VersionID:     "pub-1",
		VersionNumber: int32(n),
		ContentHash:   w.detail.Draft.ContentHash,
		PublishedAt:   "2026-01-01T00:00:00Z",
	}, nil
}

func (f *fakeStore) ListVersions(_ context.Context, ownerID, workflowID string) ([]workflowstore.VersionSummary, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w, ok := f.workflows[workflowID]
	if !ok || w.owner != ownerID {
		return nil, workflowstore.ErrNotFound
	}
	var out []workflowstore.VersionSummary
	for n := w.publishCount; n >= 1; n-- {
		out = append(out, workflowstore.VersionSummary{
			VersionID: "pub", VersionNumber: int32(n), Status: "published",
			ContentHash: "ab", CreatedBy: "ada", CreatedAt: "2026-01-01T00:00:00Z",
			NodeCount: 2, EdgeCount: 1,
		})
	}
	if w.detail.Draft != nil {
		out = append([]workflowstore.VersionSummary{{
			VersionID: w.detail.Draft.VersionID, VersionNumber: w.detail.Draft.VersionNumber,
			Status: "draft", ContentHash: w.detail.Draft.ContentHash, CreatedBy: "ada",
			CreatedAt: "2026-01-01T00:00:00Z", NodeCount: 2, EdgeCount: 1,
		}}, out...)
	}
	return out, nil
}

func (f *fakeStore) GetVersion(_ context.Context, ownerID, workflowID string, versionNumber int32) (*workflowstore.VersionDetail, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w, ok := f.workflows[workflowID]
	if !ok || w.owner != ownerID {
		return nil, workflowstore.ErrNotFound
	}
	if w.detail.Draft == nil || w.detail.Draft.VersionNumber != versionNumber {
		if versionNumber < 1 || versionNumber > int32(w.publishCount) {
			return nil, workflowstore.ErrNotFound
		}
	}
	return &workflowstore.VersionDetail{
		VersionSummary: workflowstore.VersionSummary{
			VersionID: "v", VersionNumber: versionNumber, Status: "published",
			ContentHash: "ab", CreatedBy: "ada", CreatedAt: "2026-01-01T00:00:00Z",
			NodeCount: 2, EdgeCount: 1,
		},
		Graph: w.detail.Draft.Graph,
	}, nil
}

func (f *fakeStore) CopyVersionToDraft(_ context.Context, ownerID, workflowID string, versionNumber int32, expectedRowVersion int32) (*workflowstore.DraftState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w, ok := f.workflows[workflowID]
	if !ok || w.owner != ownerID {
		return nil, workflowstore.ErrNotFound
	}
	if versionNumber < 1 || versionNumber > int32(w.publishCount) {
		return nil, workflowstore.ErrNotFound
	}
	if w.detail.Draft.RowVersion != expectedRowVersion {
		return nil, workflowstore.ErrConflict
	}
	w.detail.Draft.RowVersion++
	draft := *w.detail.Draft
	return &draft, nil
}

func (f *fakeStore) CloneWorkflow(_ context.Context, ownerID, sourceID, name string) (*workflowstore.WorkflowDetail, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	source, ok := f.workflows[sourceID]
	if !ok || source.owner != ownerID {
		return nil, workflowstore.ErrNotFound
	}
	id := fmt.Sprintf("wf-%d", f.nextID)
	f.nextID++
	detail := workflowstore.WorkflowDetail{
		WorkflowSummary: workflowstore.WorkflowSummary{
			ID: id, Name: name, Description: "Clone of " + source.detail.Name,
			Status: "draft", RowVersion: 1, UpdatedAt: "2026-01-01T00:00:00Z",
		},
		Draft: &workflowstore.DraftState{
			VersionID: "ver-" + id, VersionNumber: 1, RowVersion: 1,
			ContentHash: source.detail.Draft.ContentHash, Graph: source.detail.Draft.Graph,
		},
	}
	f.workflows[id] = &storedWorkflow{detail: detail, owner: ownerID}
	return f.workflows[id].clone(), nil
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

func TestPublishWorkflow(t *testing.T) {
	h, _ := workflowServer(t)
	id, _ := createWorkflow(t, h)

	rec := doJSON(t, h, http.MethodPost, "/api/v1/workflows/"+id+"/publish", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("publish: %d %s", rec.Code, rec.Body)
	}
	var published workflowstore.PublishedVersion
	if err := json.Unmarshal(rec.Body.Bytes(), &published); err != nil {
		t.Fatal(err)
	}
	if published.VersionNumber != 1 {
		t.Fatalf("expected version 1, got %d", published.VersionNumber)
	}

	// Publishing again produces the next monotonic version.
	rec = doJSON(t, h, http.MethodPost, "/api/v1/workflows/"+id+"/publish", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("republish: %d %s", rec.Code, rec.Body)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &published); err != nil {
		t.Fatal(err)
	}
	if published.VersionNumber != 2 {
		t.Fatalf("expected version 2, got %d", published.VersionNumber)
	}

	rec = doJSON(t, h, http.MethodPost, "/api/v1/workflows/missing/publish", "", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("publish missing: expected 404, got %d", rec.Code)
	}
}

func TestVersionHistory(t *testing.T) {
	h, _ := workflowServer(t)
	id, _ := createWorkflow(t, h)
	doJSON(t, h, http.MethodPost, "/api/v1/workflows/"+id+"/publish", "", nil)

	rec := doJSON(t, h, http.MethodGet, "/api/v1/workflows/"+id+"/versions", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list versions: %d %s", rec.Code, rec.Body)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"versionNumber":2`) || !strings.Contains(body, `"versionNumber":1`) {
		t.Fatalf("expected draft v2 and published v1 in %s", body)
	}
	if !strings.Contains(body, `"createdBy":"ada"`) {
		t.Fatalf("expected author in %s", body)
	}

	rec = doJSON(t, h, http.MethodGet, "/api/v1/workflows/"+id+"/versions/1", "", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"graph"`) {
		t.Fatalf("get version: %d %s", rec.Code, rec.Body)
	}

	rec = doJSON(t, h, http.MethodGet, "/api/v1/workflows/"+id+"/versions/99", "", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing version: expected 404, got %d", rec.Code)
	}

	rec = doJSON(t, h, http.MethodGet, "/api/v1/workflows/"+id+"/versions/abc", "", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad version number: expected 400, got %d", rec.Code)
	}
}

func TestCopyVersionToDraft(t *testing.T) {
	h, _ := workflowServer(t)
	id, _ := createWorkflow(t, h)
	doJSON(t, h, http.MethodPost, "/api/v1/workflows/"+id+"/publish", "", nil)

	// The fake reopens the draft at rowVersion 1 after publish.
	rec := doJSON(t, h, http.MethodPost, "/api/v1/workflows/"+id+"/versions/1/copy-to-draft", "", map[string]string{"If-Match": `"1"`})
	if rec.Code != http.StatusOK {
		t.Fatalf("copy: %d %s", rec.Code, rec.Body)
	}
	if rec.Header().Get("ETag") != `"2"` {
		t.Fatalf("expected advanced ETag, got %q", rec.Header().Get("ETag"))
	}

	rec = doJSON(t, h, http.MethodPost, "/api/v1/workflows/"+id+"/versions/1/copy-to-draft", "", map[string]string{"If-Match": `"1"`})
	if rec.Code != http.StatusConflict {
		t.Fatalf("stale copy: expected 409, got %d", rec.Code)
	}

	rec = doJSON(t, h, http.MethodPost, "/api/v1/workflows/"+id+"/versions/99/copy-to-draft", "", map[string]string{"If-Match": `"2"`})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing version: expected 404, got %d", rec.Code)
	}
}

func TestCloneWorkflow(t *testing.T) {
	h, _ := workflowServer(t)
	id, _ := createWorkflow(t, h)

	rec := doJSON(t, h, http.MethodPost, "/api/v1/workflows/"+id+"/clone", `{}`, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("clone: %d %s", rec.Code, rec.Body)
	}
	var clone workflowstore.WorkflowDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &clone); err != nil {
		t.Fatal(err)
	}
	if clone.ID == id {
		t.Fatal("clone must have a fresh workflow ID")
	}
	if clone.Name != "Copy of My flow" {
		t.Fatalf("unexpected default name %q", clone.Name)
	}
	if clone.Draft == nil || clone.Draft.VersionID == "" {
		t.Fatal("clone must carry a fresh draft version")
	}

	rec = doJSON(t, h, http.MethodPost, "/api/v1/workflows/"+id+"/clone", `{"name":"Named clone"}`, nil)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), "Named clone") {
		t.Fatalf("named clone: %d %s", rec.Code, rec.Body)
	}

	rec = doJSON(t, h, http.MethodPost, "/api/v1/workflows/missing/clone", `{}`, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("clone missing: expected 404, got %d", rec.Code)
	}
}

func TestListTemplates(t *testing.T) {
	h, _ := workflowServer(t)
	rec := doJSON(t, h, http.MethodGet, "/api/v1/templates", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list templates: %d %s", rec.Code, rec.Body)
	}
	body := rec.Body.String()
	for _, id := range []string{"invoice-approval", "parallel-service-request", "incident-escalation"} {
		if !strings.Contains(body, id) {
			t.Fatalf("expected template %q in %s", id, body)
		}
	}
	if !strings.Contains(body, `"source":"built-in"`) {
		t.Fatalf("expected source labeling in %s", body)
	}
}

func TestCreateWorkflowFromTemplate(t *testing.T) {
	h, _ := workflowServer(t)
	rec := doJSON(t, h, http.MethodPost, "/api/v1/workflows",
		`{"name":"From template","templateId":"invoice-approval"}`, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create from template: %d %s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "invoice.fetch") {
		t.Fatalf("expected template graph in draft, got %s", rec.Body)
	}

	rec = doJSON(t, h, http.MethodPost, "/api/v1/workflows",
		`{"name":"Bad","templateId":"nope"}`, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown template: expected 404, got %d", rec.Code)
	}

	rec = doJSON(t, h, http.MethodPost, "/api/v1/workflows",
		`{"name":"Both","templateId":"invoice-approval","graph":{}}`, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("graph+template: expected 400, got %d", rec.Code)
	}
}

func TestWorkflowRejectsOversizedBody(t *testing.T) {
	h, _ := workflowServer(t)
	// 256 KiB cap: a graph document padded beyond the limit must be rejected
	// with body_too_large before any parsing happens.
	padding := strings.Repeat(" ", 257*1024)
	body := `{"name":"` + padding + `"}`
	rec := doJSON(t, h, http.MethodPost, "/api/v1/workflows", body, nil)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d body %.200s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "body_too_large") {
		t.Fatalf("expected body_too_large code in %s", rec.Body)
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
