# ADR 0001: Monorepo with one Go module and a pnpm-managed frontend

- Status: accepted
- Date: 2026-11-22

## Context

FlowForge has three deployables (web, api, worker) plus shared contracts.
The api and worker share substantial business logic (graph validation,
execution state machine, action registry).

## Decision

Use a single repository. Go code lives in **one Go module**
(`github.com/YASSERRMD/FlowForge`) with binaries under `apps/api` and
`apps/worker` and shared packages under `internal/`. The Next.js frontend lives
in `apps/web` and is managed with pnpm. Contracts live in
`packages/contracts`.

## Consequences

- No duplicated business logic between api and worker binaries.
- One `go.mod`/`go.sum` pins the whole Go dependency set; `go build ./...`
  validates everything.
- Simple CI: one Go job, one JS job.
- Module path is tied to the repository URL; a rename requires a bulk edit.
