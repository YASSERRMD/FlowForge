package workflowstore_test

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/YASSERRMD/FlowForge/internal/db"
	"github.com/YASSERRMD/FlowForge/internal/testutil"
	"github.com/YASSERRMD/FlowForge/internal/workflowstore"
)

func freshStore(t *testing.T) *workflowstore.Store {
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
	return workflowstore.New(testutil.OpenPool(t, dsn))
}

const testGraph = `{"schemaVersion":"1","nodes":[` +
	`{"id":"start-1","kind":"start","label":"Start"},` +
	`{"id":"end-1","kind":"end","label":"End"}],` +
	`"edges":[{"id":"e-1","source":"start-1","target":"end-1"}]}`

func TestStoreRoundTrip(t *testing.T) {
	store := freshStore(t)
	ctx := context.Background()

	owner, err := store.EnsureUser(ctx, "dev-ada@dev.flowforge.local", "ada")
	if err != nil {
		t.Fatalf("ensure user: %v", err)
	}
	// Idempotent: same email returns the same user.
	again, err := store.EnsureUser(ctx, "dev-ada@dev.flowforge.local", "ada")
	if err != nil {
		t.Fatalf("ensure user again: %v", err)
	}
	if again != owner {
		t.Fatalf("expected same user, got %q vs %q", again, owner)
	}

	hash := sha256.Sum256([]byte(testGraph))
	detail, err := store.CreateWorkflow(ctx, owner, "My flow", "", json.RawMessage(testGraph), hash[:])
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if detail.Draft == nil || detail.Draft.RowVersion != 1 {
		t.Fatalf("expected draft rowVersion 1, got %+v", detail.Draft)
	}

	list, err := store.ListWorkflows(ctx, owner, 50, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].ID != detail.ID {
		t.Fatalf("list: %+v", list)
	}

	got, err := store.GetWorkflow(ctx, owner, detail.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	// JSONB normalizes key order, so compare semantically.
	var wantGraph, gotGraph any
	if err := json.Unmarshal([]byte(testGraph), &wantGraph); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(got.Draft.Graph, &gotGraph); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%v", wantGraph) != fmt.Sprintf("%v", gotGraph) {
		t.Fatalf("graph round-trip mismatch:\n%s\n%s", got.Draft.Graph, testGraph)
	}

	// Stale token conflicts; fresh token saves and advances.
	if _, err := store.SaveDraft(ctx, owner, detail.ID, json.RawMessage(testGraph), hash[:], 99); err == nil {
		t.Fatal("expected conflict on stale row version")
	}
	saved, err := store.SaveDraft(ctx, owner, detail.ID, json.RawMessage(testGraph), hash[:], 1)
	if err != nil {
		t.Fatalf("save draft: %v", err)
	}
	if saved.RowVersion != 2 {
		t.Fatalf("expected rowVersion 2 after save, got %d", saved.RowVersion)
	}

	if _, err := store.UpdateWorkflowMeta(ctx, owner, detail.ID, "Renamed", "d"); err != nil {
		t.Fatalf("update meta: %v", err)
	}

	if err := store.ArchiveWorkflow(ctx, owner, detail.ID); err != nil {
		t.Fatalf("archive: %v", err)
	}
	if _, err := store.GetWorkflow(ctx, owner, detail.ID); err == nil {
		t.Fatal("expected not found after archive")
	}
}

func TestStorePublishTransaction(t *testing.T) {
	store := freshStore(t)
	ctx := context.Background()

	owner, _ := store.EnsureUser(ctx, "dev-ada@dev.flowforge.local", "ada")
	hash := sha256.Sum256([]byte(testGraph))
	detail, err := store.CreateWorkflow(ctx, owner, "Publishable", "", json.RawMessage(testGraph), hash[:])
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	pub, err := store.Publish(ctx, owner, detail.ID)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if pub.VersionNumber != 1 {
		t.Fatalf("expected version 1, got %d", pub.VersionNumber)
	}

	// A fresh draft (v2) reopens with the published graph.
	after, err := store.GetWorkflow(ctx, owner, detail.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if after.Draft == nil || after.Draft.VersionNumber != 2 || after.Draft.RowVersion != 1 {
		t.Fatalf("expected fresh draft v2, got %+v", after.Draft)
	}
	if after.Status != "active" {
		t.Fatalf("expected active workflow, got %q", after.Status)
	}

	// Second publish supersedes the first.
	pub2, err := store.Publish(ctx, owner, detail.ID)
	if err != nil {
		t.Fatalf("republish: %v", err)
	}
	if pub2.VersionNumber != 2 {
		t.Fatalf("expected version 2, got %d", pub2.VersionNumber)
	}
}

func TestStoreOwnerIsolation(t *testing.T) {
	store := freshStore(t)
	ctx := context.Background()

	ada, _ := store.EnsureUser(ctx, "dev-ada@dev.flowforge.local", "ada")
	grace, _ := store.EnsureUser(ctx, "dev-grace@dev.flowforge.local", "grace")
	hash := sha256.Sum256([]byte(testGraph))
	detail, err := store.CreateWorkflow(ctx, ada, "Ada's flow", "", json.RawMessage(testGraph), hash[:])
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := store.GetWorkflow(ctx, grace, detail.ID); err == nil {
		t.Fatal("grace must not read ada's workflow")
	}
	if err := store.ArchiveWorkflow(ctx, grace, detail.ID); err == nil {
		t.Fatal("grace must not archive ada's workflow")
	}
}
