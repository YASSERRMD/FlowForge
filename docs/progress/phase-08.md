# Phase 08 — Go workflow execution engine: sequential DAG

Status: complete
Branch: `phase/08-execution-core-sequential`
Base: fresh `origin/main` at phase start (merge of PR #7, `3c313a1`)

## Task journal

| # | Task | Commit | Summary |
|---|------|--------|---------|
| 08.01 | Define state transitions | `3ede34b` | `internal/engine/state.go`: run states (queued/running/succeeded/failed/canceled) and step states (pending/running/succeeded/failed/skipped/canceled) with complete transition tables, terminal/active classifiers, and a test pinning the strings to the DB CHECK constraints. |
| 08.02 | Run initiation API | `9347cc9` | `Service.CreateRun`: validates ownership + published version, normalizes input (JSON object, ≤64 KiB, absent → `{}`), and in one transaction creates the queued run, writes `run.created` + outbox, and enqueues the start-node dispatch. `idempotencyKey` replays return the earlier run (200 vs 201). `POST /api/v1/workflows/{id}/runs` + OpenAPI + parity. |
| 08.03 | Durable queue claim | `a9b680b` | `internal/engine/queue.go`: typed wrapper over `scheduled_tasks` — `Claim` (FOR UPDATE SKIP LOCKED + bounded leases), `Renew`, `Complete` (lease-holder only), `Fail` (backoff → pending, exhausted → dead), `EnqueueStep` with dedupe keys (`run:<id>:node:<id>`). Race tests: 2 workers × 20 tasks, exactly-once claims, lease-expiry reclaim, dedupe collapse. |
| 08.04 | Worker lifecycle | `c1ed54a` | `internal/engine/worker.go`: poll loop with configurable interval, semaphore-bounded concurrency, per-task lease renewal goroutine, graceful shutdown (stop claiming, wait up to ShutdownTimeout, leases expire naturally), exponential backoff capped at 300 s. `apps/worker/main.go`: config, Postgres pool, health server (`/healthz`, `/readyz` with DB ping) on `WORKER_HEALTH_ADDR`, SIGINT/SIGTERM handling. |
| 08.05 | Start node execution | `2df862e` | `internal/engine/executor.go`: `ExecuteNode` locks the run row (FOR UPDATE), loads the immutable published graph, moves queued→running with `run.started`, records the attempt (create → start with input snapshot → succeed with output), emits ordered events + outbox in the same transaction, and dispatches default-handle successors. Start node: validates the stored input and hands it to successors, no side effects. Missing node → durable run failure. Terminal-run dispatch → no-op. |
| 08.06 | Deterministic action executor | `2ba4812` | `internal/engine/actions`: isolated registry with typed inputs/outputs; `mock.echo` (pass-through envelope `{ok, echo}`) and `mock.fail` (synthetic failures). All actions pure: no clock, no randomness, no I/O. Unknown types fail the step with `unknown_action` and the run durably. Executor accepts injected registries for tests. |
| 08.07 | End node completion | `d41835e` | End node succeeds only after durable state agrees: zero active attempts and zero other pending dispatches for the run → `FinishRun(succeeded)` + `run.succeeded`. Otherwise the run stays running (matters once phases 09–10 add branches). |
| 08.08 | Persist node attempt records | `072bc52` | Per-attempt input/output/error persisted with 256 KiB payload caps: oversized configured input fails the run as `input_too_large` without a poisoned attempt row; oversized action output fails the step as `output_too_large`. Tests cover the full lifecycle fields, failure error fields, and both caps. |
| 08.09 | Append ordered events transactionally | `410722a` | Every state change writes its `run_events` row and outbox mirror in the same transaction. Tests: gapless 1..N sequences, event↔outbox count/order/type agreement, canceled execution leaves zero partial traces and the run recovers. Fixed `run.created` outbox payload to carry top-level `runId` like all other events. |
| 08.10 | Run detail API | `2d56a9e` | `Service.GetRunDetail` (owner-scoped): status, timestamps, duration, per-node latest state with structured attempts (input/output/error/timestamps). `GET /api/v1/runs/{runID}` + OpenAPI + parity + handler tests. |
| 08.11 | Sequential recovery primitives | `8d51c6e` | Worker-driven invoice-like gate test (worker → queue → dispatcher → executor → success, exact causal order, no queue residue); worker-restart reclaim (crashed claim expires, second worker completes, exactly one attempt per node); state persists across a fresh connection pool. |
| — | Formatting fix | `fb5c3fc`-style fix commit | gofmt correction on `internal/config/config.go`. |

## Gate results (branch tip)

- `gofmt -l internal apps` — clean
- `go vet ./...` — pass
- `go build ./...` — pass
- `go test -race ./...` (with `FLOWFORGE_TEST_DATABASE_URL`) — all packages pass
- `git diff --check origin/main...HEAD` — clean; 12 commits, all authored+committed by `YASSERRMD <arafath.yasser@gmail.com>`
- No `apps/web` changes this phase (web gate runs in CI regardless)

## End-to-end smoke (real processes, local Postgres + Redis)

API (`:18099`) and worker (`smoke-w1`, health `:18098`) both ready. Via the API only:
created workflow → saved invoice-like linear graph (fetch → approve → notify) → published v1 →
`POST /workflows/{id}/runs` → run `de642361…` reached `succeeded` in ~209 ms with one succeeded
attempt per node and structured inputs/outputs on the detail endpoint. Killed the worker, restarted
as `smoke-w2`: run detail unchanged, event stream intact (`run.created … run.succeeded`, 13 events,
gapless), new worker healthy.

## Design decisions

- **Deterministic step failure ≠ task failure.** Bad configs, unknown actions and oversized payloads fail the step/run durably and complete the queue task; only infrastructure errors return to the queue for retry. Retries of business failures arrive with phase 11 policies.
- **One transaction per node execution.** Attempt lifecycle, events, outbox and successor dispatches commit or roll back together — a crash mid-node leaves no partial state (proven by test).
- **End-node completion checks durable state, not graph reachability**, so the rule survives parallel branches unchanged (phase 10).
- **Branching kinds fail loudly** (`unsupported_node_kind`) until phases 09/10 implement them; the sequential gate uses linear graphs.

## Acceptance gate

- Sequential invoice-like workflow runs fully via worker and API — ✔ (integration test + live smoke)
- Events and state persist across process restart — ✔ (restart test + live smoke)
- Claims are race-safe — ✔ (SKIP LOCKED race test, duplicate-claim and lease-expiry tests)
