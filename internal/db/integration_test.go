package db_test

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/YASSERRMD/FlowForge/internal/db"
	"github.com/YASSERRMD/FlowForge/internal/db/dbgen"
	"github.com/YASSERRMD/FlowForge/internal/testutil"
)

func strptr(s string) *string { return &s }

// freshSchema creates an isolated schema inside the disposable test
// database, migrates it fully, and returns both a stdlib handle (for
// trigger-level assertions) and the typed query layer. Skips when no
// test DB is configured.
func freshSchema(t *testing.T) (*sql.DB, *pgxpool.Pool, *dbgen.Queries) {
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
	return sqlDB, pool, dbgen.New(pool)
}

func seedRun(t *testing.T, ctx context.Context, q *dbgen.Queries, name string) (dbgen.WorkflowRun, dbgen.WorkflowVersion) {
	t.Helper()
	user, err := q.CreateUser(ctx, dbgen.CreateUserParams{
		Email: name + "@example.com", DisplayName: name, PasswordHash: "x", Role: "developer",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	wf, err := q.CreateWorkflow(ctx, dbgen.CreateWorkflowParams{
		OwnerID: user.ID, Name: name, Description: "",
	})
	if err != nil {
		t.Fatalf("create workflow: %v", err)
	}
	v, err := q.CreateWorkflowVersion(ctx, dbgen.CreateWorkflowVersionParams{
		WorkflowID: wf.ID, VersionNumber: 1, GraphJson: []byte(`{"nodes":[]}`),
		ContentHash: make([]byte, 32), CreatedBy: user.ID,
	})
	if err != nil {
		t.Fatalf("create version: %v", err)
	}
	run, err := q.CreateRun(ctx, dbgen.CreateRunParams{
		WorkflowID: wf.ID, WorkflowVersionID: v.ID, Variables: []byte(`{}`),
		TriggerType: "manual",
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	return run, v
}

func TestMigrationsReachLatestVersion(t *testing.T) {
	sqlDB, _, _ := freshSchema(t)
	v, err := db.MigrateVersion(sqlDB)
	if err != nil {
		t.Fatal(err)
	}
	migrations, err := goose.CollectMigrations("migrations", 0, goose.MaxVersion)
	if err != nil {
		t.Fatal(err)
	}
	if want := int64(len(migrations)); v != want {
		t.Fatalf("version = %d, want %d", v, want)
	}
}

func TestMigrationDownAndUpRestoresSchema(t *testing.T) {
	sqlDB, _, _ := freshSchema(t)

	snapshot := func() map[string]bool {
		rows, err := sqlDB.Query(`
			SELECT table_name || '.' || column_name || ':' || data_type
			FROM information_schema.columns
			WHERE table_schema = 'public'
			  AND table_name <> 'goose_db_version'
			ORDER BY 1`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		cols := map[string]bool{}
		for rows.Next() {
			var c string
			if err := rows.Scan(&c); err != nil {
				t.Fatal(err)
			}
			cols[c] = true
		}
		return cols
	}

	before := snapshot()

	for {
		v, err := db.MigrateVersion(sqlDB)
		if err != nil {
			t.Fatal(err)
		}
		if v == 0 {
			break
		}
		if err := db.MigrateDown(sqlDB); err != nil {
			t.Fatalf("migrate down from %d: %v", v, err)
		}
	}
	var tables int
	if err := sqlDB.QueryRow(`
		SELECT count(*) FROM information_schema.tables
		WHERE table_schema = 'public'
		  AND table_name <> 'goose_db_version'`).Scan(&tables); err != nil {
		t.Fatal(err)
	}
	if tables != 0 {
		t.Fatalf("%d tables remain after full rollback", tables)
	}

	if err := db.MigrateUp(sqlDB); err != nil {
		t.Fatalf("re-migrate: %v", err)
	}
	after := snapshot()
	if len(before) != len(after) {
		t.Fatalf("column count differs after down/up: %d vs %d", len(before), len(after))
	}
	for c := range after {
		if !before[c] {
			t.Errorf("column after re-migrate but not before: %s", c)
		}
	}
}

func TestUniqueEmailConstraint(t *testing.T) {
	_, _, q := freshSchema(t)
	ctx := context.Background()
	params := dbgen.CreateUserParams{
		Email: "ada@example.com", DisplayName: "Ada", PasswordHash: "x", Role: "developer",
	}
	if _, err := q.CreateUser(ctx, params); err != nil {
		t.Fatalf("create user: %v", err)
	}
	if _, err := q.CreateUser(ctx, params); err == nil {
		t.Fatal("duplicate email accepted")
	}
}

func TestOneDraftPerWorkflow(t *testing.T) {
	_, _, q := freshSchema(t)
	ctx := context.Background()
	user, _ := q.CreateUser(ctx, dbgen.CreateUserParams{
		Email: "d@example.com", DisplayName: "D", PasswordHash: "x", Role: "developer",
	})
	wf, _ := q.CreateWorkflow(ctx, dbgen.CreateWorkflowParams{
		OwnerID: user.ID, Name: "wf", Description: "",
	})
	mk := func(n int32) error {
		_, err := q.CreateWorkflowVersion(ctx, dbgen.CreateWorkflowVersionParams{
			WorkflowID: wf.ID, VersionNumber: n, GraphJson: []byte(`{}`),
			ContentHash: make([]byte, 32), CreatedBy: user.ID,
		})
		return err
	}
	if err := mk(1); err != nil {
		t.Fatalf("first draft: %v", err)
	}
	if err := mk(2); err == nil {
		t.Fatal("second concurrent draft accepted")
	}
}

func TestPublishedVersionIsImmutable(t *testing.T) {
	sqlDB, _, q := freshSchema(t)
	ctx := context.Background()
	_, v := seedRun(t, ctx, q, "immut")
	if _, err := q.PublishVersion(ctx, v.ID); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if _, err := sqlDB.ExecContext(ctx,
		"UPDATE workflow_versions SET graph_json = '{}'::jsonb WHERE id = $1", v.ID); err == nil {
		t.Fatal("published version accepted mutation")
	}
	if _, err := sqlDB.ExecContext(ctx,
		"DELETE FROM workflow_versions WHERE id = $1", v.ID); err == nil {
		t.Fatal("published version accepted delete")
	}
}

func TestRunRejectsForeignVersion(t *testing.T) {
	_, _, q := freshSchema(t)
	ctx := context.Background()
	run1, _ := seedRun(t, ctx, q, "owner-a")
	_, v2 := seedRun(t, ctx, q, "owner-b")
	if _, err := q.CreateRun(ctx, dbgen.CreateRunParams{
		WorkflowID: run1.WorkflowID, WorkflowVersionID: v2.ID,
		Variables: []byte(`{}`), TriggerType: "manual",
	}); err == nil {
		t.Fatal("run accepted a version belonging to another workflow")
	}
}

func TestOptimisticRowVersionBumps(t *testing.T) {
	_, _, q := freshSchema(t)
	ctx := context.Background()
	user, _ := q.CreateUser(ctx, dbgen.CreateUserParams{
		Email: "ov@example.com", DisplayName: "OV", PasswordHash: "x", Role: "developer",
	})
	wf, _ := q.CreateWorkflow(ctx, dbgen.CreateWorkflowParams{
		OwnerID: user.ID, Name: "ov-wf", Description: "",
	})
	updated, err := q.UpdateWorkflowMeta(ctx, dbgen.UpdateWorkflowMetaParams{
		ID: wf.ID, Name: "ov-wf-2", Description: "changed",
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.RowVersion != wf.RowVersion+1 {
		t.Fatalf("row_version = %d, want %d", updated.RowVersion, wf.RowVersion+1)
	}
}

func TestRunEventSequencesAreOrdered(t *testing.T) {
	_, _, q := freshSchema(t)
	ctx := context.Background()
	run, _ := seedRun(t, ctx, q, "events")

	for i := 0; i < 5; i++ {
		ev, err := q.AppendRunEvent(ctx, dbgen.AppendRunEventParams{
			RunID: run.ID, EventType: "step", Payload: []byte(`{"i":1}`),
		})
		if err != nil {
			t.Fatalf("append event %d: %v", i, err)
		}
		if ev.Sequence != int64(i+1) {
			t.Fatalf("sequence = %d, want %d", ev.Sequence, i+1)
		}
	}

	events, err := q.ListRunEvents(ctx, dbgen.ListRunEventsParams{
		RunID: run.ID, Sequence: 0, Limit: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 5 {
		t.Fatalf("events = %d", len(events))
	}
	seqs := make([]int64, len(events))
	for i, e := range events {
		seqs[i] = e.Sequence
	}
	if !slices.IsSorted(seqs) {
		t.Fatalf("events not ordered: %v", seqs)
	}
}

func TestConcurrentTaskClaimsAreDisjoint(t *testing.T) {
	_, pool, q := freshSchema(t)
	ctx := context.Background()

	for i := 0; i < 20; i++ {
		if _, err := q.EnqueueTask(ctx, dbgen.EnqueueTaskParams{
			TaskType: "step", DedupeKey: fmt.Sprintf("task-%d", i),
			Payload: []byte(`{}`), MaxAttempts: 3,
		}); err != nil {
			t.Fatalf("enqueue %d: %v", i, err)
		}
	}

	claims := map[string][]string{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for w := 0; w < 2; w++ {
		wg.Add(1)
		go func(worker string) {
			defer wg.Done()
			own := dbgen.New(pool)
			for {
				tasks, err := own.ClaimTasks(ctx, dbgen.ClaimTasksParams{
					LeasedBy: strptr(worker), Limit: 5, Secs: 30,
				})
				if err != nil {
					t.Errorf("claim: %v", err)
					return
				}
				if len(tasks) == 0 {
					return
				}
				mu.Lock()
				for _, task := range tasks {
					claims[worker] = append(claims[worker], task.DedupeKey)
				}
				mu.Unlock()
			}
		}(fmt.Sprintf("worker-%d", w))
	}
	wg.Wait()

	seen := map[string]string{}
	total := 0
	for worker, keys := range claims {
		for _, k := range keys {
			total++
			if prev, dup := seen[k]; dup {
				t.Fatalf("task %s claimed by both %s and %s", k, prev, worker)
			}
			seen[k] = worker
		}
	}
	if total != 20 {
		t.Fatalf("claimed %d tasks, want 20", total)
	}
}

func TestFailTaskRetriesThenDies(t *testing.T) {
	_, _, q := freshSchema(t)
	ctx := context.Background()

	task, err := q.EnqueueTask(ctx, dbgen.EnqueueTaskParams{
		TaskType: "step", DedupeKey: "flaky", Payload: []byte(`{}`), MaxAttempts: 2,
	})
	if err != nil {
		t.Fatal(err)
	}

	for attempt := 1; attempt <= 2; attempt++ {
		claimed, err := q.ClaimTasks(ctx, dbgen.ClaimTasksParams{
			LeasedBy: strptr("w1"), Limit: 1, Secs: 30,
		})
		if err != nil || len(claimed) != 1 {
			t.Fatalf("claim attempt %d: %v (%d tasks)", attempt, err, len(claimed))
		}
		failed, err := q.FailTask(ctx, dbgen.FailTaskParams{
			ID: claimed[0].ID, LeasedBy: strptr("w1"), LastError: strptr("boom"), Secs: 0,
		})
		if err != nil {
			t.Fatalf("fail attempt %d: %v", attempt, err)
		}
		if attempt < 2 && failed.Status != "pending" {
			t.Fatalf("status after attempt %d = %s, want pending", attempt, failed.Status)
		}
		if attempt == 2 && failed.Status != "dead" {
			t.Fatalf("status after max attempts = %s, want dead", failed.Status)
		}
	}

	claimed, err := q.ClaimTasks(ctx, dbgen.ClaimTasksParams{
		LeasedBy: strptr("w1"), Limit: 10, Secs: 30,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range claimed {
		if c.ID == task.ID {
			t.Fatal("dead task was claimed")
		}
	}
}
