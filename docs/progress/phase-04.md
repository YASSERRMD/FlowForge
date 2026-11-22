# Phase 04 — PostgreSQL schema, migrations and data-access layer

Branch: `phase/04-database-schema-access` (from merged `origin/main` after PR #3)

## Task journal

| # | Task | Commit | Evidence |
|---|------|--------|----------|
| 04.01 | Add migration runner | `d6332fe` | goose v3 with migrations embedded in the binary (`internal/db/migrations.go`, `//go:embed`); `api migrate [-status|-down]` subcommand; `-down` refused in production; verified live against Postgres 17 container (version 8 → 9 later) |
| 04.02 | Create identity schema | `a032c43` | `users` (citext email unique, role check) + `sessions` (SHA-256 token_hash, expiry, revoke; covering partial indexes) |
| 04.03 | Create workflow schema | `11ccbab` | `workflows` with owner FK, lifecycle check, `UNIQUE(owner_id, name)` |
| 04.04 | Create immutable versions schema | `4022b18` | `workflow_versions`: version_number unique per workflow, content_hash, one-draft partial unique index, trigger rejecting UPDATE/DELETE of published+superseded rows |
| 04.05 | Create runs schema | `888a25f` | `workflow_runs`: status machine, variables JSONB, cancellation timestamp, idempotency-key partial unique index |
| 04.06 | Create step attempt schema | `dc2a171` | `step_attempts`: `UNIQUE(run_id, node_id, attempt_number)`, retry counters, error code/message, unique idempotency token |
| 04.07 | Create event and outbox schema | `e028836` | `run_events` with PK `(run_id, sequence)` total order; `outbox_messages` with unpublished partial index |
| 04.08 | Create worker lease schema | `226e070` | `scheduled_tasks`: dedupe unique key, lease owner/expiry, attempts/max_attempts, runnable + lease-expiry partial indexes for `FOR UPDATE SKIP LOCKED` claiming |
| 04.09 | Generate sqlc queries | `d22bea2` | sqlc v1.31.1 + pgx/v5; 4 query files (users, workflows, runs, tasks); generated `internal/db/dbgen` compiles; codegen determinism enforced in CI |
| 04.10 | Add database constraints | `cf4e833` | `00009_constraints.sql`: row_version optimistic-concurrency triggers, value checks (hash lengths, positive numbers), cross-FK trigger (run's version must belong to its workflow), hot-path indexes |
| 04.11 | Test migrations and repositories | `cd124f7` | 10 integration tests against disposable per-test databases: full down→up schema-equivalence cycle, unique email, one-draft rule, published immutability, foreign-version rejection, row_version bump, event ordering, concurrent SKIP LOCKED claims disjoint (2 workers × 20 tasks), retry→dead path; CI go job gained a Postgres 17 service so these run on every PR |

## Migration verification (actual output)

```
OK   00001_baseline.sql ... OK   00009_constraints.sql
goose: successfully migrated database to version: 9
```

Down/up cycle: `TestMigrationDownAndUpRestoresSchema` rolls all 9 migrations
back (0 non-goose tables remain) and re-applies; column snapshots identical.

## Phase gate results

- `gofmt -l .` clean; `go vet ./...` clean; `go build ./...` OK
- `go test ./...` and `go test -race ./...` pass **with** `FLOWFORGE_TEST_DATABASE_URL` set (integration tests active)
- Web untouched; `pnpm lint`/`tsc`/`test`/`build` re-verified at gate
- CI: postgres:17-alpine service added; sqlc codegen determinism check added
- `git diff --check` clean; 11 commits `phase 04: *`; author=committer YASSERRMD

## Design decisions

- Migrations embedded under `internal/db/migrations/` (self-contained binaries); `db/queries/` holds sqlc definitions; layout note updated in `db/README.md`.
- sqlc parses goose annotations natively — no schema duplication.
- Integration tests skip cleanly when `FLOWFORGE_TEST_DATABASE_URL` is unset, so unit-only environments stay green.
