# FlowForge

FlowForge is a self-hosted, visual workflow designer with a durable Go execution
platform and real-time run debugging.

- `apps/web` — Next.js App Router frontend (React, TypeScript, `@xyflow/react`).
- `apps/api` — Go REST + WebSocket service (chi, pgx, sqlc).
- `apps/worker` — standalone Go worker process that executes workflow runs.
- `packages/contracts` — OpenAPI and JSON Schema contracts shared across stacks.
- `db/migrations` — forward-only PostgreSQL migrations (goose).
- `db/queries` — sqlc query definitions.
- `infra` — Docker Compose topology and deployment configuration.
- `docs/adr` — architecture decision records.
- `docs/progress` — execution journal and phase evidence.
- `tests/e2e` — Playwright browser tests.
- `scripts` — local development and verification helpers.

See `docs/` for architecture, onboarding and phase evidence.
