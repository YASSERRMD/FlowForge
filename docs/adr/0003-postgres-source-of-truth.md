# ADR 0003: PostgreSQL is the sole source of truth for runs

- Status: accepted
- Date: 2026-11-22

## Context

A durable execution engine needs a trustworthy record of what happened, even
across worker crashes and message-broker outages.

## Decision

PostgreSQL stores every authoritative fact: workflow definitions, immutable
published versions, runs, node states, step attempts, the runnable-task queue
with leases, the ordered per-run event log, the notification outbox and audit
events. Cross-table state changes (state transition + attempt row + new tasks +
events + outbox) commit in **one transaction**.

## Consequences

- A run is fully reconstructable from PostgreSQL alone.
- `FOR UPDATE SKIP LOCKED` gives race-safe task claims without extra infra.
- The database is the bottleneck for scheduling throughput; bounded worker
  concurrency and indexing are required.
