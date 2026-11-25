package engine_test

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/YASSERRMD/FlowForge/internal/db"
	"github.com/YASSERRMD/FlowForge/internal/db/dbgen"
	"github.com/YASSERRMD/FlowForge/internal/engine"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YASSERRMD/FlowForge/internal/testutil"
	"github.com/YASSERRMD/FlowForge/internal/workflowstore"
)

// sequentialGraph is a linear start → action → end flow, the shape phase 08
// executes end to end.
const sequentialGraph = `{"schemaVersion":"1","nodes":[` +
	`{"id":"start-1","kind":"start","label":"Start","position":{"x":0,"y":0}},` +
	`{"id":"action-1","kind":"action","label":"Work","position":{"x":0,"y":120},` +
	`"config":{"actionType":"mock.echo","params":{"value":"hello"}}},` +
	`{"id":"end-1","kind":"end","label":"End","position":{"x":0,"y":240}}],` +
	`"edges":[` +
	`{"id":"e1","source":"start-1","target":"action-1"},` +
	`{"id":"e2","source":"action-1","target":"end-1"}]}`

type fixture struct {
	t       *testing.T
	sqlDB   *sql.DB
	pool    *pgxpool.Pool
	store   *workflowstore.Store
	svc     *engine.Service
	q       *dbgen.Queries
	ownerID string
	ctx     context.Context
}

func newFixture(t *testing.T) *fixture {
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
	f := &fixture{
		t:     t,
		sqlDB: sqlDB,
		pool:  pool,
		store: workflowstore.New(pool),
		svc:   engine.NewService(pool),
		q:     dbgen.New(pool),
		ctx:   context.Background(),
	}
	owner, err := f.store.EnsureUser(f.ctx, "dev-ada@dev.flowforge.local", "ada")
	if err != nil {
		t.Fatalf("ensure user: %v", err)
	}
	f.ownerID = owner
	return f
}

// createPublished makes a workflow with the given graph and publishes it.
func (f *fixture) createPublished(name, graphJSON string) *workflowstore.WorkflowDetail {
	f.t.Helper()
	hash := sha256.Sum256([]byte(graphJSON))
	detail, err := f.store.CreateWorkflow(f.ctx, f.ownerID, name, "",
		json.RawMessage(graphJSON), hash[:])
	if err != nil {
		f.t.Fatalf("create workflow: %v", err)
	}
	if _, err := f.store.Publish(f.ctx, f.ownerID, detail.ID); err != nil {
		f.t.Fatalf("publish: %v", err)
	}
	return detail
}

func TestCreateRunQueuesAgainstPublishedVersion(t *testing.T) {
	f := newFixture(t)
	detail := f.createPublished("Invoice-like", sequentialGraph)

	created, err := f.svc.CreateRun(f.ctx, f.ownerID, detail.ID,
		json.RawMessage(`{"invoice_id":"inv-1"}`), "")
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	if created.ReplayedFrom {
		t.Fatal("first creation must not be a replay")
	}
	if created.Run.Status != string(engine.RunQueued) {
		t.Fatalf("run status = %q, want queued", created.Run.Status)
	}
	if created.StartNodeID != "start-1" {
		t.Fatalf("start node = %q, want start-1", created.StartNodeID)
	}
	var vars map[string]any
	if err := json.Unmarshal(created.Run.Variables, &vars); err != nil || vars["invoice_id"] != "inv-1" {
		t.Fatalf("variables = %s", created.Run.Variables)
	}

	// The start node dispatch was enqueued atomically with the run.
	var taskCount int
	runID := created.Run.ID
	if err := f.sqlDB.QueryRow(
		`SELECT count(*) FROM scheduled_tasks
		 WHERE task_type = 'execute_node' AND dedupe_key = $1`,
		"run:"+uuidString(t, runID.Bytes[:])+":node:start-1").Scan(&taskCount); err != nil {
		t.Fatal(err)
	}
	if taskCount != 1 {
		t.Fatalf("expected 1 start dispatch task, got %d", taskCount)
	}
}

func TestCreateRunRequiresPublishedVersion(t *testing.T) {
	f := newFixture(t)
	hash := sha256.Sum256([]byte(sequentialGraph))
	detail, err := f.store.CreateWorkflow(f.ctx, f.ownerID, "DraftOnly", "",
		json.RawMessage(sequentialGraph), hash[:])
	if err != nil {
		t.Fatalf("create workflow: %v", err)
	}
	if _, err := f.svc.CreateRun(f.ctx, f.ownerID, detail.ID, nil, ""); !errors.Is(err, engine.ErrNoPublishedVersion) {
		t.Fatalf("expected ErrNoPublishedVersion, got %v", err)
	}
}

func TestCreateRunEnforcesOwnership(t *testing.T) {
	f := newFixture(t)
	detail := f.createPublished("Owned", sequentialGraph)
	other, err := f.store.EnsureUser(f.ctx, "dev-eve@dev.flowforge.local", "eve")
	if err != nil {
		t.Fatalf("ensure user: %v", err)
	}
	if _, err := f.svc.CreateRun(f.ctx, other, detail.ID, nil, ""); !errors.Is(err, engine.ErrWorkflowNotFound) {
		t.Fatalf("expected ErrWorkflowNotFound, got %v", err)
	}
}

func TestCreateRunValidatesInput(t *testing.T) {
	f := newFixture(t)
	detail := f.createPublished("Inputs", sequentialGraph)

	for name, input := range map[string]json.RawMessage{
		"array":  json.RawMessage(`[1,2]`),
		"scalar": json.RawMessage(`42`),
		"null":   json.RawMessage(`null`),
	} {
		if _, err := f.svc.CreateRun(f.ctx, f.ownerID, detail.ID, input, ""); !errors.Is(err, engine.ErrInvalidInput) {
			t.Errorf("%s: expected ErrInvalidInput, got %v", name, err)
		}
	}

	// Absent input normalizes to an empty object.
	created, err := f.svc.CreateRun(f.ctx, f.ownerID, detail.ID, nil, "")
	if err != nil {
		t.Fatalf("nil input: %v", err)
	}
	if string(created.Run.Variables) != `{}` {
		t.Fatalf("nil input should normalize to {}, got %s", created.Run.Variables)
	}
}

func TestCreateRunIdempotencyKeyReplays(t *testing.T) {
	f := newFixture(t)
	detail := f.createPublished("Idempotent", sequentialGraph)

	first, err := f.svc.CreateRun(f.ctx, f.ownerID, detail.ID, nil, "key-1")
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := f.svc.CreateRun(f.ctx, f.ownerID, detail.ID, nil, "key-1")
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if !second.ReplayedFrom {
		t.Fatal("second create with same key must replay")
	}
	if first.Run.ID != second.Run.ID {
		t.Fatalf("replay returned a different run: %s vs %s", first.Run.ID.Bytes, second.Run.ID.Bytes)
	}

	// Exactly one run and one task exist.
	var runs, tasks int
	if err := f.sqlDB.QueryRow(`SELECT count(*) FROM workflow_runs`).Scan(&runs); err != nil {
		t.Fatal(err)
	}
	if err := f.sqlDB.QueryRow(`SELECT count(*) FROM scheduled_tasks`).Scan(&tasks); err != nil {
		t.Fatal(err)
	}
	if runs != 1 || tasks != 1 {
		t.Fatalf("expected 1 run and 1 task, got %d and %d", runs, tasks)
	}
}

func uuidString(t *testing.T, b []byte) string {
	t.Helper()
	if len(b) != 16 {
		t.Fatalf("uuid bytes = %d", len(b))
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func TestGetRunDetailReturnsNodeStates(t *testing.T) {
	f := newFixture(t)
	detail := f.createPublished("Detail", sequentialGraph)
	created, err := f.svc.CreateRun(f.ctx, f.ownerID, detail.ID, jsonRaw(`{"a":1}`), "")
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	runID := uuidString(t, created.Run.ID.Bytes[:])
	executeDrain(t, f, runID)

	d, err := f.svc.GetRunDetail(f.ctx, f.ownerID, runID)
	if err != nil {
		t.Fatalf("get detail: %v", err)
	}
	if d.Status != "succeeded" || d.VersionNumber != 1 {
		t.Fatalf("status=%q version=%d", d.Status, d.VersionNumber)
	}
	if len(d.Nodes) != 3 {
		t.Fatalf("expected 3 node states, got %d", len(d.Nodes))
	}
	for _, n := range d.Nodes {
		if n.Status != "succeeded" || len(n.Attempts) != 1 {
			t.Fatalf("node %s: status=%q attempts=%d", n.NodeID, n.Status, len(n.Attempts))
		}
		if n.Attempts[0].StartedAt == "" || n.Attempts[0].FinishedAt == "" {
			t.Fatalf("node %s missing timestamps: %+v", n.NodeID, n.Attempts[0])
		}
	}
	if d.StartedAt == "" || d.FinishedAt == "" {
		t.Fatal("run timestamps missing")
	}
}

func TestGetRunDetailScopesToOwner(t *testing.T) {
	f := newFixture(t)
	detail := f.createPublished("Scoped", sequentialGraph)
	created, err := f.svc.CreateRun(f.ctx, f.ownerID, detail.ID, nil, "")
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	runID := uuidString(t, created.Run.ID.Bytes[:])

	other, err := f.store.EnsureUser(f.ctx, "dev-eve@dev.flowforge.local", "eve")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.GetRunDetail(f.ctx, other, runID); !errors.Is(err, engine.ErrRunNotFound) {
		t.Fatalf("expected ErrRunNotFound, got %v", err)
	}
}
