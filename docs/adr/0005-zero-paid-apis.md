# ADR 0005: Zero paid or external service dependencies

- Status: accepted
- Date: 2026-11-22

## Context

The project must boot with `docker compose up` on a clean machine with no
accounts, API keys or network-restricted services.

## Decision

The full topology is web + api + worker + postgres + redis, all open-source
containers. Demonstration actions (invoice validation, approvals, service
checks, notifications) are **simulated by deterministic built-in executors**.
No real payment, email, SMS or other external side effect is performed by the
demo templates.

## Consequences

- Reproducible offline-friendly demos and tests.
- Exactly-once external side effects are explicitly out of scope; real
  integrations must implement idempotency keys or compensations (documented in
  the reliability notes).
