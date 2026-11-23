package httpserver

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/YASSERRMD/FlowForge/internal/db"
	"github.com/YASSERRMD/FlowForge/internal/testutil"
	"github.com/YASSERRMD/FlowForge/internal/workflowstore"
)

// liveServer boots the real handler stack over a freshly migrated database.
func liveServer(t *testing.T) http.Handler {
	t.Helper()
	dsn := testutil.FreshDatabase(t)
	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open scoped db: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	if err := db.MigrateUp(sqlDB); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	pool := testutil.OpenPool(t, dsn)

	cfg := testConfig()
	cfg.DevAuthEnabled = true
	return New(cfg, testLogger(), WithWorkflowStore(workflowstore.New(pool))).Handler()
}

func integrationGraph() string {
	return `{"schemaVersion":"1","nodes":[` +
		`{"id":"start-1","kind":"start","label":"Start","position":{"x":0,"y":0}},` +
		`{"id":"action-1","kind":"action","label":"Fetch","position":{"x":40,"y":120},` +
		`"config":{"actionType":"http.request","params":{"url":"https://example.com"},"futureKey":true}},` +
		`{"id":"condition-1","kind":"condition","label":"OK?","position":{"x":40,"y":240},` +
		`"config":{"expression":"$steps.action-1.output.status == 200"}},` +
		`{"id":"end-1","kind":"end","label":"End","position":{"x":0,"y":360}}],` +
		`"edges":[` +
		`{"id":"e-1","source":"start-1","sourceHandle":null,"target":"action-1","targetHandle":null},` +
		`{"id":"e-2","source":"action-1","sourceHandle":null,"target":"condition-1","targetHandle":null},` +
		`{"id":"e-3","source":"condition-1","sourceHandle":"true","target":"end-1","targetHandle":null,"label":"true"}]}`
}

// TestDraftPersistenceRoundTrip is the phase gate: a draft round-trips
// through API and database without loss, version tokens advance, and
// malformed input is rejected consistently.
func TestDraftPersistenceRoundTrip(t *testing.T) {
	h := liveServer(t)

	create := doJSON(t, h, http.MethodPost, "/api/v1/workflows",
		`{"name":"Roundtrip","graph":`+integrationGraph()+`}`, nil)
	if create.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", create.Code, create.Body)
	}
	var detail workflowstore.WorkflowDetail
	if err := json.Unmarshal(create.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	etag := create.Header().Get("ETag")
	if etag != `"1"` {
		t.Fatalf("expected ETag \"1\", got %q", etag)
	}

	// Read back: every node, edge and config key must survive — including the
	// unknown "futureKey" the contract requires to be preserved.
	got := doJSON(t, h, http.MethodGet, "/api/v1/workflows/"+detail.ID, "", nil)
	if got.Code != http.StatusOK {
		t.Fatalf("get: %d %s", got.Code, got.Body)
	}
	var readBack workflowstore.WorkflowDetail
	if err := json.Unmarshal(got.Body.Bytes(), &readBack); err != nil {
		t.Fatal(err)
	}
	var want, have map[string]any
	if err := json.Unmarshal([]byte(integrationGraph()), &want); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(readBack.Draft.Graph, &have); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%v", want) != fmt.Sprintf("%v", have) {
		t.Fatalf("graph not preserved:\nwant %v\nhave %v", want, have)
	}

	// Save with the read token, then confirm the token advanced.
	save := doJSON(t, h, http.MethodPut, "/api/v1/workflows/"+detail.ID+"/draft",
		`{"graph":`+integrationGraph()+`}`, map[string]string{"If-Match": etag})
	if save.Code != http.StatusOK {
		t.Fatalf("save: %d %s", save.Code, save.Body)
	}
	if save.Header().Get("ETag") != `"2"` {
		t.Fatalf("expected ETag \"2\" after save, got %q", save.Header().Get("ETag"))
	}

	// The original token is now stale.
	stale := doJSON(t, h, http.MethodPut, "/api/v1/workflows/"+detail.ID+"/draft",
		`{"graph":`+integrationGraph()+`}`, map[string]string{"If-Match": etag})
	if stale.Code != http.StatusConflict {
		t.Fatalf("stale save: expected 409, got %d %s", stale.Code, stale.Body)
	}
}

func TestDraftRejectsMalformedGraphs(t *testing.T) {
	h := liveServer(t)
	create := doJSON(t, h, http.MethodPost, "/api/v1/workflows", `{"name":"Guards"}`, nil)
	if create.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", create.Code, create.Body)
	}
	var detail workflowstore.WorkflowDetail
	if err := json.Unmarshal(create.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name  string
		graph string
		code  string
	}{
		{"cycle", `{"schemaVersion":"1","nodes":[{"id":"start-1","kind":"start","label":"S"},{"id":"action-1","kind":"action","label":"A","config":{"actionType":"x"}},{"id":"end-1","kind":"end","label":"E"}],"edges":[{"id":"e1","source":"start-1","target":"action-1"},{"id":"e2","source":"action-1","target":"action-1"},{"id":"e3","source":"action-1","target":"end-1"}]}`, "self_loop"},
		{"bad port", `{"schemaVersion":"1","nodes":[{"id":"start-1","kind":"start","label":"S"},{"id":"condition-1","kind":"condition","label":"C","config":{"expression":"$trigger.a > 1"}},{"id":"end-1","kind":"end","label":"E"}],"edges":[{"id":"e1","source":"start-1","target":"condition-1"},{"id":"e2","source":"condition-1","sourceHandle":"maybe","target":"end-1"}]}`, "invalid_source_handle"},
		{"bad expression ref", `{"schemaVersion":"1","nodes":[{"id":"start-1","kind":"start","label":"S"},{"id":"condition-1","kind":"condition","label":"C","config":{"expression":"bogus == 1"}},{"id":"end-1","kind":"end","label":"E"}],"edges":[{"id":"e1","source":"start-1","target":"condition-1"},{"id":"e2","source":"condition-1","sourceHandle":"true","target":"end-1"}]}`, "config"},
		{"dangling edge", `{"schemaVersion":"1","nodes":[{"id":"start-1","kind":"start","label":"S"},{"id":"end-1","kind":"end","label":"E"}],"edges":[{"id":"e1","source":"start-1","target":"ghost"}]}`, "dangling_edge"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := doJSON(t, h, http.MethodPut, "/api/v1/workflows/"+detail.ID+"/draft",
				`{"graph":`+tc.graph+`}`, map[string]string{"If-Match": `"1"`})
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("expected 422, got %d %s", rec.Code, rec.Body)
			}
			if !strings.Contains(rec.Body.String(), tc.code) {
				t.Fatalf("expected issue code %q in %s", tc.code, rec.Body)
			}
		})
	}
}
