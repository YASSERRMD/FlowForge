# Phase 06 — Graph validation and authoring persistence

Branch: `phase/06-graph-validation-authoring` (from merged `origin/main` after PR #5)

## Task journal

| # | Task | Commit | Evidence |
|---|------|--------|----------|
| 06.01 | Publish graph JSON schema | `d1f006c` | `packages/contracts/graph-schema.json` (JSON Schema 2020-12, `schemaVersion "1"`, 200/400 limits), TS mirror `apps/web/lib/contracts/graph.ts`, migration policy in `docs/contracts/graph.md` |
| 06.02 | Node configuration forms | `8aa280c` | Schema-driven descriptors `lib/editor/config-schema.ts` (action/delay/condition/fork/join), side panel `components/editor/config-panel.tsx` with accessible FormField wiring; store `updateNodeConfig`; 10 vitest |
| 06.03 | Client graph validation | `d22efab` | `lib/editor/validate.ts` (16 issue codes incl. missing start/end, orphans, dangling edges, ports, cycles, size limits) + `lib/editor/convert.ts` React Flow ↔ contract conversion; 12 vitest |
| 06.04 | Backend graph validation (Go) | `cd2771a` | `internal/graph/types.go` + `validate.go` mirroring the client rule-for-rule (DAG acyclicity, reachability, fan rules, port compatibility, size limits); 10 Go tests incl. canonical byte-stable round-trip |
| 06.05 | Condition grammar | `7a1fc83` | `internal/graph/expr.go` parser (comparisons, `$trigger.*` / `$steps.<id>.output.*` refs, and/or/not/parens, typed operand rules, 1-based positional errors), TS mirror `lib/editor/expr.ts` with parity tests, `docs/contracts/conditions.md`; wired into both validators and the config panel |
| 06.06 | Draft CRUD API | `400de6c` | Migration 00010 (`workflow_versions.row_version` + bump trigger), sqlc queries (`GetWorkflowForOwner`, `UpdateDraftGraph`, `EnsureUser`), `internal/workflowstore` over pgxpool, REST under `/api/v1/workflows` behind RequireAuth with dev-identity→user bootstrap, ETag/If-Match optimistic saves, 422 with full issue list, OpenAPI updated + parity test; 8 handler tests (fake store) + live curl smoke (201/200/409/422/204/404) |
| 06.07 | Typed frontend API client | `27972f5` (+ fix `214da4d`) | `apps/web/lib/api/client.ts` mirroring the OpenAPI contract with uniform error-envelope mapping (`ApiError.code/status/fields`, `isConflict`); 7 vitest |
| 06.08 | Draft autosave + conflicts | `f773b6c` | `lib/editor/persistence.ts`: debounced (800 ms) saves with rowVersion token, 409 → autosave stops + conflict banner with explicit reload (no silent overwrite), error surfacing, flush; `PersistentEditor` wired into `/workflows/[id]`; 6 vitest |
| 06.09 | Validation inspector | `e908c29` | Collapsible panel `components/editor/validation-inspector.tsx`, live revalidation, click-to-focus (select + `setCenter`); 3 component tests |
| 06.10 | Persistence roundtrip tests | `48fa813` | Client conversion stability (6 vitest) + **HTTP-level integration tests against real migrated Postgres**: graph with unknown config key round-trips losslessly, ETag `"1"→"2"`, stale token 409, cycles/ports/bad refs rejected with issue codes |
| 06.11 | Error and import bounds | `1c6a7a3` | `lib/editor/import.ts` (256 KiB cap, JSON/shape/version/size/structure errors with clear messages, warnings pass-through), Import/Export UI with sonner toasts; server-side 256 KiB body cap test (413 `body_too_large`); 10 vitest + 1 Go test |
| — | Fix-forward | `7dec11a` | Bound default `fetch` (browser "Illegal invocation" found by e2e); e2e selectors scoped to canvas nodes after inspector echoes labels |

## Phase gate results

- Go: `gofmt -l` clean, `go vet ./...` clean, `go build ./...` ok,
  `go test ./...` green **with `FLOWFORGE_TEST_DATABASE_URL`** (fresh DB per
  test, migrations to version 10), `go test -race` green on graph/httpserver/workflowstore
- Web: `pnpm lint` clean, `tsc --noEmit` clean, **146/146 vitest (16 files)**,
  `pnpm build` ok, **5/5 Playwright e2e** against the production bundle
- Gate: draft round-trips API+DB without loss (unknown config keys preserved);
  cycles/invalid ports/bad references rejected with identical issue codes client and server
- `git diff --check` clean; 13 commits `phase 06: *`; author=committer YASSERRMD

## Design decisions

- **Optimistic token**: `workflow_versions.row_version` (new migration 00010 + shared bump trigger) exposed as a quoted ETag; `PUT …/draft` requires `If-Match`; mismatch → 409. No silent last-write-wins anywhere.
- **Dev identity bootstrap**: the dev authenticator's subject maps to an idempotent `EnsureUser` upsert (`dev-<name>@dev.flowforge.local`, unusable `!dev-account` password hash), so every workflow row has a real owner FK; production keeps failing closed (no authenticator → 401).
- **JSONB note**: Postgres normalizes key order; losslessness is asserted semantically and drift is covered by the SHA-256 content hash.
- Client and server validators/grammars are deliberately duplicated code with parity tests, per the contract docs — no shared runtime across the Go/TS boundary.
