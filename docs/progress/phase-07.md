# Phase 07 — Publishing, immutable versions and starter templates

Status: complete
Branch: `phase/07-workflow-versioning-templates`
Base: fresh `origin/main` at phase start (merge of PR #6, `0133911`)

## Task journal

| # | Task | Commit | Summary |
|---|------|--------|---------|
| 07.01 | Publish transaction | `e1b822c` | Migration `00011_publish_transition.sql` amends the phase-04 `reject_version_mutation` trigger so published→superseded is allowed only when `graph_json`, `content_hash` and `version_number` are unchanged. `Store.Publish` runs one transaction: publish draft → supersede prior published → set workflow current version → activate workflow → create the next draft with the same graph. `POST /api/v1/workflows/{id}/publish` re-validates the draft graph server-side before publishing. |
| 07.02 | Immutable versions | `833fba9` | Integration tests prove direct UPDATE/DELETE on published rows is rejected by trigger, supersession preserves the graph bytes, and the terminal status set is exactly {superseded, published, draft}. |
| 07.03 | Version history API | `f7bf300` | `ListVersionsWithAuthor` / `GetVersionWithAuthor` queries join `users` for `created_by_name`. `GET /workflows/{id}/versions` and `GET /workflows/{id}/versions/{n}` return `VersionSummary` records with node/edge counts, content hash and author as diff metadata. |
| 07.04 | Version comparison UI | `2b1337f` | `lib/editor/diff.ts` computes semantic node/edge add/remove/change diffs with `summarizeDiff`. Client gains `publish` / `listVersions` / `getVersion`. New `components/workflows/version-history.tsx` and page `app/(dashboard)/workflows/[id]/versions/page.tsx`. |
| 07.05 | Copy from published | `fc40315` | `Store.CopyVersionToDraft` (If-Match row-version guarded) + `POST /workflows/{id}/versions/{n}/copy-to-draft` + client `copyVersionToDraft`. |
| 07.06 | Clone API | `e653cd5` | `Store.CloneWorkflow` prefers the published graph, falls back to draft; fresh entity IDs; caller owns the clone; default name "Copy of X". `POST /workflows/{id}/clone` + client `cloneWorkflow`. |
| 07.07 | Invoice approval template | `db59201` | `internal/templates` registry with deterministic Go-literal graph: threshold condition over invoice amount with mock.* actions. |
| 07.08 | Parallel service request template | `1dc81cb` | Fork (branchCount 3) → three parallel checks → join → final condition. |
| 07.09 | Incident escalation template | `1585464` | Retry/onFailure params, acknowledgement condition, escalation path, `mock.event.emit`. All three templates pass `graph.Validate`. |
| 07.10 | Template gallery | `07ad90a` | `GET /api/v1/templates` (RequireAuth) returns all templates including graphs. `templateId` on create-workflow (mutually exclusive with inline `graph`; unknown id → 404). `components/workflows/template-gallery.tsx` with search and step preview; templates page rewritten; 3 new component tests. |
| 07.11 | Immutability + template integrity tests | `f25d8db` | `publish_concurrency_integration_test.go`: 6 racing publishes serialize into distinct monotonic version numbers with exactly 1 draft, 1 published, k−1 superseded; a second status-guarded UPDATE on the same draft row affects 0 rows. `TestTemplatesMatchContractSchema` cross-checks template graphs against the contract schema constants and asserts every action type is namespaced `mock.*`. |

## Gate results (all run on the branch tip `f25d8db`)

- `gofmt -l internal apps` — clean
- `go vet ./...` — pass
- `go build ./...` — pass
- `go test ./...` (with `FLOWFORGE_TEST_DATABASE_URL`) — pass
- `go test -race ./internal/graph/ ./internal/httpserver/ ./internal/workflowstore/ ./internal/templates/` — pass
- `pnpm run lint` — pass
- `pnpm exec tsc --noEmit` — pass
- `pnpm run test` — 153/153 pass
- `pnpm run build` — pass (routes incl. `/workflows/[id]/versions`, `/templates`)
- Playwright e2e (production build, port 3100) — 5/5 pass
- `git diff --check origin/main...HEAD` — clean
- Authorship: every commit authored and committed by `YASSERRMD <arafath.yasser@gmail.com>`

## Design decisions

- **Publish supersedes, never mutates.** The same trigger that rejects edits to published versions permits the published→superseded transition only when the graph payload is untouched, so supersession is a status flip and nothing else.
- **Concurrency outcome is a chain, not a winner-take-all.** Each publish mints the next draft, so concurrent publishers either lose with `ErrNoDraft` (same-draft row-lock race) or publish the successor draft. The invariant asserted under `-race` is: distinct monotonic version numbers, exactly one live draft, exactly one published head.
- **Templates are code, not seed data.** Built-in templates are Go literals validated against the same `graph.Validate` as user graphs and shipped over the API, so the gallery needs no database seeding and stays deterministic across environments.

## Phase-gate acceptance (from the phase spec)

- Templates open in the editor and can be cloned — via gallery "Use template" and `POST /workflows/{id}/clone`. ✔
- Published snapshots cannot change — enforced by trigger, proven by integration tests. ✔
- Every run can reference an exact published version — `workflows.current_version_id` and immutable `workflow_versions` rows provide the stable pointer; execution engine consumes it in phase 08+. ✔
