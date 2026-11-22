# Developer onboarding

## Prerequisites

| Tool | Version used | Check |
|---|---|---|
| Go | 1.25.x | `go version` |
| Node.js | 24.x | `node --version` |
| pnpm | 10.22.x | `pnpm --version` |
| Docker | 29.x | `docker --version` |
| Git | any recent | `git --version` |

Pinned dependency versions live in `go.mod` / `go.sum` and
`apps/web/pnpm-lock.yaml`. The pnpm version is pinned via the
`packageManager` field in `apps/web/package.json`.

## First run

```bash
git clone <repo-url> && cd FlowForge
cp .env.example .env          # safe local defaults, no real secrets
make setup                    # go mod download + pnpm install
make verify                   # fmt check, vet, tests, race, builds
```

Individual loops:

```bash
go test ./...                 # Go unit tests
go build ./...                # compile api + worker + libraries
cd apps/web && pnpm dev       # Next.js dev server on :3000
cd apps/web && pnpm build     # production build
```

`make up` / `make down` manage the Docker Compose topology once
`infra/docker-compose.yml` exists (phase 15 adds the full topology; earlier
phases add it incrementally).

## Repository map

See the root `README.md` and `docs/architecture.md` for the container diagram
and component ownership rules. Decision records live in `docs/adr/`.

## Code ownership and boundaries

- `apps/api`, `apps/worker`, `internal/`, `db/` — Go. PostgreSQL is the only
  source of truth; Redis is transient notification only (ADR 0003, 0004).
- `apps/web` — TypeScript/React. The UI edits and inspects; it never executes
  or simulates workflow logic.
- `packages/contracts` — OpenAPI + JSON Schema. Both stacks validate against
  these artifacts; do not let Go models, SQL and TS types drift.

## Branch and commit workflow

This project follows a strict phase-based Git workflow:

- Work happens on `phase/XX-short-slug` branches cut from the latest merged
  `origin/main`. Never commit directly to `main`.
- One atomic commit per task, immediately after the task's checks pass.
  Commit subjects follow `phase 01: <short description>`.
- Commit author **and** committer must be `YASSERRMD
  <arafath.yasser@gmail.com>` (local repo config).
- Phases close by pushing the branch, opening a PR to `main`, waiting for CI,
  and merging with a **merge commit** (`gh pr merge --merge --delete-branch`).
  Squash and rebase merges are forbidden.
- Fixes after a merge go on `fix/<slug>` branches with `fix: ...` subjects.

## Local checks before pushing

```bash
make verify          # full gate
git diff --check     # whitespace
git status           # tree must be clean
```

## Troubleshooting

- **`pnpm install` fails on lockfile**: run `pnpm install --frozen-lockfile`
  to confirm the lockfile matches `package.json`; update intentionally with
  `pnpm install` and commit the new lockfile.
- **ESLint flat-config errors**: the config is `apps/web/eslint.config.mjs`
  (ESLint 10 flat config). `.eslintrc.*` files are not used.
- **Go build cache issues**: `go clean -cache` is safe; dependencies are
  pinned in `go.sum`.
- **gofmt failures in CI**: run `gofmt -w .` and commit the result.
