package workflowstore_test

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/YASSERRMD/FlowForge/internal/db"
	"github.com/YASSERRMD/FlowForge/internal/testutil"
	"github.com/YASSERRMD/FlowForge/internal/workflowstore"
)

// TestConcurrentPublishesAreSerialized fires concurrent publishes at one
// workflow. Each publish creates the next draft, so publishes form a
// serialized chain: the integrity invariant is that every success yields a
// distinct monotonic version number and exactly one draft survives.
func TestConcurrentPublishesAreSerialized(t *testing.T) {
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
	detail, err := store.CreateWorkflow(ctx, owner, "Concurrent", "", json.RawMessage(testGraph), hash[:])
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	const racers = 6
	start := make(chan struct{})
	var wg sync.WaitGroup
	numbers := make([]int32, racers)
	errs := make([]error, racers)
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			pub, err := store.Publish(ctx, owner, detail.ID)
			errs[i] = err
			if err == nil {
				numbers[i] = pub.VersionNumber
			}
		}(i)
	}
	close(start)
	wg.Wait()

	seen := map[int32]bool{}
	succeeded := 0
	for i := 0; i < racers; i++ {
		if errs[i] != nil {
			// The only legitimate failure: this racer lost the row-lock race
			// for the same draft the winner published.
			if !errors.Is(errs[i], workflowstore.ErrNoDraft) {
				t.Fatalf("publish %d failed unexpectedly: %v", i, errs[i])
			}
			continue
		}
		succeeded++
		if seen[numbers[i]] {
			t.Fatalf("duplicate published version number %d", numbers[i])
		}
		seen[numbers[i]] = true
	}
	if succeeded < 1 {
		t.Fatal("at least one publish must succeed")
	}
	for n := int32(1); n <= int32(succeeded); n++ {
		if !seen[n] {
			t.Fatalf("version %d missing from published set %v", n, seen)
		}
	}

	// Each publish supersedes the previously published version, so the chain
	// settles into: 1 draft, 1 published, (succeeded-1) superseded.
	var drafts, published, superseded int
	if err := sqlDB.QueryRow(
		`SELECT count(*) FROM workflow_versions WHERE workflow_id = $1 AND status = 'draft'`,
		detail.ID).Scan(&drafts); err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.QueryRow(
		`SELECT count(*) FROM workflow_versions WHERE workflow_id = $1 AND status = 'published'`,
		detail.ID).Scan(&published); err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.QueryRow(
		`SELECT count(*) FROM workflow_versions WHERE workflow_id = $1 AND status = 'superseded'`,
		detail.ID).Scan(&superseded); err != nil {
		t.Fatal(err)
	}
	if drafts != 1 {
		t.Fatalf("expected exactly 1 surviving draft, got %d", drafts)
	}
	if published != 1 {
		t.Fatalf("expected exactly 1 published version, got %d", published)
	}
	if superseded != succeeded-1 {
		t.Fatalf("expected %d superseded versions, got %d", succeeded-1, superseded)
	}
}

// TestPublishSameDraftTwiceFails proves the same draft row cannot be
// published twice: the status-guarded UPDATE admits exactly one winner.
func TestPublishSameDraftTwiceFails(t *testing.T) {
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

	owner, _ := store.EnsureUser(ctx, "dev-ada@dev.flowforge.local", "ada")
	hash := sha256.Sum256([]byte(testGraph))
	detail, err := store.CreateWorkflow(ctx, owner, "Once", "", json.RawMessage(testGraph), hash[:])
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Two transactions race the same draft row's status-guarded update.
	tag1, err := sqlDB.Exec(
		`UPDATE workflow_versions SET status = 'published', published_at = now()
		 WHERE id = $1 AND status = 'draft'`, detail.Draft.VersionID)
	if err != nil {
		t.Fatalf("first update: %v", err)
	}
	tag2, err := sqlDB.Exec(
		`UPDATE workflow_versions SET status = 'published', published_at = now()
		 WHERE id = $1 AND status = 'draft'`, detail.Draft.VersionID)
	if err != nil {
		t.Fatalf("second update should error only via trigger, got %v", err)
	}
	n1, _ := tag1.RowsAffected()
	n2, _ := tag2.RowsAffected()
	if n1 != 1 || n2 != 0 {
		t.Fatalf("expected 1 winner and 1 loser, got %d and %d", n1, n2)
	}
}
