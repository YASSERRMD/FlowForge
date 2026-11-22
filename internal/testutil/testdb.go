// Package testutil provides shared test helpers, including access to the
// disposable integration-test database.
package testutil

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"

	"context"
)

var dbNameRe = regexp.MustCompile(`^[a-z0-9_]+$`)

// TestDSN returns the disposable test database DSN from
// FLOWFORGE_TEST_DATABASE_URL, skipping the test when unset.
func TestDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("FLOWFORGE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("FLOWFORGE_TEST_DATABASE_URL not set; skipping integration test")
	}
	return dsn
}

// FreshDatabase creates a uniquely named database in the disposable
// Postgres instance and returns a DSN pointing at it, with cleanup that
// drops it. Per-database isolation keeps extension and goose bookkeeping
// semantics identical to production.
func FreshDatabase(t *testing.T) (scopedDSN string) {
	t.Helper()
	dsn := TestDSN(t)

	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatal(err)
	}
	name := "flowforge_test_" + hex.EncodeToString(b[:])
	if !dbNameRe.MatchString(name) {
		t.Fatalf("generated database name %q fails safety check", name)
	}

	admin, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open admin connection: %v", err)
	}
	defer admin.Close()
	if _, err := admin.Exec("CREATE DATABASE " + name); err != nil {
		t.Fatalf("create test database: %v", err)
	}
	t.Cleanup(func() {
		cleanup, err := sql.Open("pgx", dsn)
		if err != nil {
			return
		}
		defer cleanup.Close()
		_, _ = cleanup.Exec("DROP DATABASE IF EXISTS " + name + " WITH (FORCE)")
	})

	return replaceDBName(dsn, name)
}

// replaceDBName swaps the database name in a postgres DSN, supporting both
// URL (postgres://host/db?params) and keyword (host=... dbname=...) forms.
func replaceDBName(dsn, name string) string {
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		rest := dsn[strings.Index(dsn, "://")+3:]
		slash := strings.Index(rest, "/")
		head := dsn[:strings.Index(dsn, "://")+3] + rest[:slash+1]
		tail := rest[slash+1:]
		if q := strings.Index(tail, "?"); q >= 0 {
			return head + name + tail[q:]
		}
		return head + name
	}
	fields := strings.Fields(dsn)
	for i, f := range fields {
		if strings.HasPrefix(f, "dbname=") {
			fields[i] = "dbname=" + name
			return strings.Join(fields, " ")
		}
	}
	return dsn + " dbname=" + name
}

// OpenPool opens a pgx pool against the given DSN and pings it.
func OpenPool(t *testing.T, dsn string) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		t.Fatalf("ping pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}
