# FlowForge architecture

This document fixes the architectural boundaries of the system. Changes to
these boundaries require an ADR under `docs/adr/`.

## C4 level 1 — system context

```text
[User (browser)]
   │  HTTPS: REST /api/v1, WebSocket run stream
   ▼
[FlowForge system]  ── self-hosted, single Docker Compose deployment
   ├─ PostgreSQL (sole source of truth)
   └─ Redis (transient pub/sub notifications only)
```

FlowForge has no dependency on cloud or paid external services. Demonstration
actions are simulated locally; no real payment, email or SMS is ever sent.

## C4 level 2 — containers

```text
apps/web (Next.js)
   │  REST /api/v1 (JSON)            ┌───────────────────────────┐
   ├────────────────────────────────▶│ apps/api (Go, chi)        │
   │  WebSocket /runs/{id}/stream    │  authoring, validation,   │
   └────────────────────────────────▶│  publish, launch, cancel, │
                                    │  event replay             │
                                    └─────────────┬─────────────┘
                                                  │ pgx + sqlc, ACID tx
                                    ┌─────────────▼─────────────┐
                                    │ PostgreSQL                │
                                    │  users, sessions,         │
                                    │  workflows, versions,     │
                                    │  runs, node states,       │
                                    │  attempts, runnable_tasks,│
                                    │  run_events, outbox,audit │
                                    └─────────────▲─────────────┘
                                                  │ FOR UPDATE SKIP LOCKED
                                    ┌─────────────┴─────────────┐
                                    │ apps/worker (Go)          │
                                    │  claims tasks, executes   │
                                    │  deterministic actions,   │
                                    │  commits state + events   │
                                    └─────────────┬─────────────┘
                                                  │ outbox publish
                                    ┌─────────────▼─────────────┐
                                    │ Redis pub/sub (transient) │
                                    └───────────────────────────┘
```

## Component ownership rules

- **React/Next.js (apps/web)** edits and inspects. It never executes workflow
  logic and never simulates server state with client-side timers.
- **Go API (apps/api)** owns all writes to workflow definitions, publishing and
  run lifecycle requests, plus read models and the WebSocket event stream.
- **Go worker (apps/worker)** owns execution: task claims, node executors,
  retries, cancellation and recovery. Business logic lives in shared `internal/`
  packages; it is never duplicated between the api and worker binaries.
- **PostgreSQL** is the only authoritative store. Any fact about a run that
  matters must be recoverable from PostgreSQL alone.
- **Redis** carries transient notifications and wake-ups. The system stays
  correct (only slower to observe) if Redis is down; committed events replay
  from the `event_outbox` table.

## Hard invariants

1. v1 graphs are DAGs; cycles are rejected at validation time.
2. Every run pins an immutable published workflow version and its content hash.
3. State transitions, attempts, new runnable tasks and ordered events commit in
   one PostgreSQL transaction.
4. Conditions use a typed, bounded expression grammar; no `eval` of user input
   in any language.
5. Retries are at-least-once attempts; built-in actions are deterministic and
   idempotent.
6. Local development and default production configuration never silently grant
   anonymous write access.
