package workflowstore_test

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"

	"github.com/YASSERRMD/FlowForge/internal/db"
	"github.com/YASSERRMD/FlowForge/internal/testutil"
	"github.com/YASSERRMD/FlowForge/internal/workflowstore"
)

// setupPublished returns a store with one workflow that has a published v1
// and an open draft v2, plus a stdlib handle for raw assertions.
func setupPublished(t *testing.T) (*workflowstore.Store, *sql.DB, string) {
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

	store := workflowstore.New(testutil.OpenPool(t, dsn))
	ctx := context.Background()
	owner, err := store.EnsureUser(ctx, "dev-ada@dev.flowforge.local", "ada")
	if err != nil {
		t.Fatalf("ensure user: %v", err)
	}
	hash := sha256.Sum256([]byte(testGraph))
	detail, err := store.CreateWorkflow(ctx, owner, "Immutable", "", json.RawMessage(testGraph), hash[:])
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := store.Publish(ctx, owner, detail.ID); err != nil {
		t.Fatalf("publish: %v", err)
	}
	return store, sqlDB, detail.ID
}

func TestPublishedGraphIsImmutableAtDatabaseLevel(t *testing.T) {
	_, sqlDB, workflowID := setupPublished(t)

	// Direct graph mutation on the published row must be rejected.
	_, err := sqlDB.Exec(
		`UPDATE workflow_versions SET graph_json = '{}'::jsonb
		 WHERE workflow_id = $1 AND status = 'published'`, workflowID)
	if err == nil || !strings.Contains(err.Error(), "immutable") {
		t.Fatalf("expected immutability error, got %v", err)
	}

	// Deletes of published rows must be rejected too.
	_, err = sqlDB.Exec(
		`DELETE FROM workflow_versions WHERE workflow_id = $1 AND status = 'published'`, workflowID)
	if err == nil || !strings.Contains(err.Error(), "immutable") {
		t.Fatalf("expected immutability error on delete, got %v", err)
	}
}

func TestSupersedePreservesPublishedGraph(t *testing.T) {
	store, sqlDB, workflowID := setupPublished(t)
	ctx := context.Background()

	// Publishing v2 supersedes v1 through the one permitted transition.
	owner := mustOwner(t, store)
	if _, err := store.Publish(ctx, owner, workflowID); err != nil {
		t.Fatalf("second publish: %v", err)
	}

	var statuses []string
	rows, err := sqlDB.Query(
		`SELECT status FROM workflow_versions WHERE workflow_id = $1 ORDER BY version_number`, workflowID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		statuses = append(statuses, s)
	}
	want := []string{"superseded", "published", "draft"}
	if strings.Join(statuses, ",") != strings.Join(want, ",") {
		t.Fatalf("expected %v, got %v", want, statuses)
	}

	// The superseded row still holds the exact original graph.
	var graph []byte
	if err := sqlDB.QueryRow(
		`SELECT graph_json FROM workflow_versions WHERE workflow_id = $1 AND version_number = 1`,
		workflowID).Scan(&graph); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(graph, &got); err != nil {
		t.Fatal(err)
	}
	if got["schemaVersion"] != "1" {
		t.Fatalf("superseded graph corrupted: %s", graph)
	}
}

// mustOwner resolves ada's dev user id for reuse in follow-up calls.
func mustOwner(t *testing.T, store *workflowstore.Store) string {
	t.Helper()
	owner, err := store.EnsureUser(context.Background(), "dev-ada@dev.flowforge.local", "ada")
	if err != nil {
		t.Fatalf("ensure user: %v", err)
	}
	return owner
}
