# Phase 09 — Conditions, variable context and mock actions

Status: complete (pending PR merge)
Branch: `phase/09-conditions-context-actions`
Base: `main` after phase 08 merge (`22a9541`)

## Task journal

| # | Task | Commit | Summary |
|---|------|--------|---------|
| 09.01 | Typed run context | `3ad8d6c` | `internal/graph/context.go`: versioned `Context` holding trigger input and per-node step outputs; `MergeStep` (retry-replace), `TriggerValue`, `StepValue`, `HasStep`, and generic dotted-path `LookupPath` supporting object keys and array indexes. |
| 09.02 | Expression parser/evaluator | `41bd722` | `internal/graph/eval.go`: recursive-descent evaluator for `$trigger.*` / `$steps.<node>.output.*` references, string/number/bool/null literals, comparisons, `and`/`or` with short-circuit, and type errors as structured `EvalError`. Numeric coercion keeps JSON numbers comparable. |
| 09.03 | Reject unsafe expressions | `8b964ce` | `unsafeSegment` rejects `__proto__`, `prototype` and `constructor` path segments anywhere in a reference; pinned by `unsafe_test.go` and documented in `docs/contracts/conditions.md`. |
| 09.04 | Execute condition branches | `84e00a4` | Executor builds the run context from the trigger plus succeeded outputs, evaluates condition nodes to a boolean, routes only the selected `true`/`false` handle, and records the branch on `step.succeeded`. `params.go` resolves `$`-references in action params (exact-reference only, missing → `null`, nested objects/arrays). |
| 09.05 | Skip semantics | `f805d87` | `propagateSkips` marks the untaken subtree `skipped` with reason `branch_not_taken`; skips are transitive and join-safe (a node with any active predecessor stays active). Emits `step.skipped`. Documented plus cascade/merge tests. |
| 09.06 | Deterministic action catalog | `48bf6bb` | `internal/engine/actions/catalog.go`: invoice fetch/validate/approve/reject, review request, email/status-page notify, pager page/escalate, event emit, request load/fulfill/reject, inventory/credit/address checks. Pure functions with FNV-derived amounts, `FAIL`-prefixed identifiers force failed checks. Registered in `Default()`. |
| 09.07 | Catalog determinism tests | `1f1def3` | Pins repeatability, defaults, validation issues, decision outputs, notification channels, forced check failures, and malformed-input rejection; registry `Types()` stays sorted. |
| 09.08 | Persist in-app notifications | `9e4ee5c` | Migration `00012_notifications.sql` + `InsertNotification`/`ListNotificationsByRun`. Successful `mock.notify.*` actions write an owner-scoped record (channel/recipient/subject/payload) inside the node transaction; nothing external is sent. |
| 09.09 | Delay action | `8b080ac` | Delay nodes emit `{durationMs, wakeAt}` and schedule successors at the wake time (`EnqueueStepAt`) via the injectable executor clock; invalid durations fail as `invalid_delay_config`. Tests pin scheduling and successor propagation. |
| 09.10 | Context inspection endpoints | `48424aa` | `GET /api/v1/runs/{runID}/context` (trigger + succeeded step outputs) and `GET /api/v1/runs/{runID}/notifications`, both owner-scoped (foreign IDs → 404). OpenAPI updated + parity test. |
| 09.11 | Acceptance paths | `91b1383` | End-to-end template tests: invoice auto-approve vs review branches (notifications to finance/requester), incident escalation (`statuspage` record), and catalog coverage of every template action. |
| — | Repository hygiene | `4058c20` | Removed the accidentally committed 16 MB `api` binary and ignored local `/api` + `/worker` build outputs. |

## Gate results (branch tip)

- `gofmt -l internal apps` — clean
- `go vet ./...` — pass
- `go build ./...` — pass
- `go test -race ./...` with `FLOWFORGE_TEST_DATABASE_URL` — all packages pass
- `sqlc generate` — no diff in `internal/db/dbgen`
- OpenAPI spec validates and covers the new routes
- 13 commits authored by `YASSERRMD <arafath.yasser@gmail.com>`; no `temp/` files tracked

## Design decisions

- **Expressions are data, not code.** A small evaluator with an explicit grammar avoids arbitrary execution and keeps replay deterministic; unsafe property segments are rejected before evaluation.
- **Params resolve exact references only.** A value that is exactly `$trigger.x` or `$steps.n.output.x` is substituted (including into nested arrays/objects); embedded references stay literal to avoid surprising string interpolation.
- **Skips are derived from durable state.** Skip propagation consults the latest attempt status per node and only skips nodes whose predecessors are all skipped, so reconverging branches do not drop work. Phase 10 can layer join barriers on the same rule.
- **Notifications are records, not deliveries.** Mock notification actions persist auditable rows in the same transaction as the step; the platform never contacts email/SMS providers.
- **Delays schedule successors**, not the delay node itself, and use an injectable clock so tests and replays are exact.

## Acceptance gate

- Branching workflows execute correctly (both branches, skip propagation) — ✔
- Expressions handle references, comparisons and boolean logic; unsafe paths rejected — ✔
- Invoice approval and incident escalation templates run end to end with persisted notifications — ✔
- Context and notifications are inspectable over owner-scoped APIs — ✔
- No external services required for any mock action — ✔
