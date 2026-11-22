# Phase 03 — Go service and API foundation

Branch: `phase/03-go-api-foundation` (from merged `origin/main` after PR #2)

## Task journal

| # | Task | Commit | Evidence |
|---|------|--------|----------|
| 03.01 | Configure chi router | `4fdbaaa` | chi v5 router, `/api/v1` group, graceful shutdown with drain; `TestGracefulShutdownOnContextCancel` passes |
| 03.02 | Implement typed service config | `fe7487e` | HTTP timeouts, max body, `AUTH_DEV_MODE` (rejected in production); 8 config tests pass |
| 03.03 | Add structured request logging | `f7c1db6` | request-ID middleware (honors + echoes `X-Request-ID`), slog JSON lines with route pattern/latency/status; credential-looking query params redacted (test proves no leak) |
| 03.04 | Add panic recovery middleware | `9d29c9d` | panic → safe 500 with correlation ID; panic detail + stack only in server log (test proves no client leak) |
| 03.05 | Establish uniform error contract | `aab602e` (+gofmt `781599b`) | `internal/apierror`: 11 domain codes, status mapping, field errors, single JSON envelope; unknown errors collapsed to generic 500 |
| 03.06 | Add health and readiness probes | `f2b1c39` | `/healthz` dependency-free; `/readyz` runs registered checks concurrently with bounded timeout, 503 + per-dep body on failure; real TCP dial checks wired for postgres/redis in `main.go` |
| 03.07 | Implement json validation | `97bebc1` | strict `DecodeJSON`: body cap (413), malformed (400), unknown fields listed (400), trailing data rejected, type errors → field errors; 7 edge-case tests |
| 03.08 | Add initial session boundary | `e9e14d4` | `Authenticator` interface, `Identity`, `RequireAuth`; dev boundary only with `APP_ENV=development`+`AUTH_DEV_MODE=true`; nil authenticator fails closed 401 (production test proves dev header ignored) |
| 03.09 | Add openapi skeleton | `04874ba` | `docs/api/openapi.yaml` validated by kin-openapi in CI tests; spec/route parity test; `make openapi-check` wired into `make verify` |
| 03.10 | Add middleware tests | `d58dcf4` | full-stack tests: correlation flow, panic→500 through chain, one log line per request, live httptest server, contract-shaped error body |
| 03.11 | Wire api smoke workflow | `f3be09c` | `scripts/smoke-api.sh` run against the real binary: 7/7 checks PASS (`SMOKE OK`); curl reference added to `docs/development.md` |

## Smoke output (actual run, 2026-11-22)

```
PASS GET /healthz -> 200 (200)
PASS GET /api/v1/ping -> 200 (200)
PASS GET /api/v1/me without credentials -> 401 (401)
PASS GET /api/v1/me with dev identity -> 200 (200)
PASS /api/v1/me returns dev identity
PASS /readyz -> 503 with structured body   (no local postgres/redis running — correct fail)
PASS X-Request-ID echoed (smoke-corr-1)
SMOKE OK
```

## Phase gate results

- `gofmt -l .` — clean
- `go vet ./...` — clean
- `go test ./...` — all packages pass (config, buildinfo, apierror, httpserver, middleware)
- `go test -race ./...` — pass
- `go build ./...` — pass
- `make openapi-check` — spec valid, spec/route parity holds
- Web unchanged; `pnpm lint`/`tsc`/`test`/`build` re-verified at gate
- `git diff --check` — clean; 12 commits, all `phase 03: ...`, author=committer `YASSERRMD <arafath.yasser@gmail.com>`
- No unauthenticated production writes: production config yields no authenticator → every guarded route fails closed 401 (tested)

## Notable design decisions

- Middleware order: RequestID → RequestLogger → Recoverer (IDs available to logs and panic responses).
- Auth lives in `httpserver` (not `middleware`) to avoid an import cycle with `apierror`.
- Readiness uses TCP dial checks until the real pgx/redis drivers land (phase 04+); the `Checker` registry accepts any `CheckFunc`.
