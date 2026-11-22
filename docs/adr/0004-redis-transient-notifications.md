# ADR 0004: Redis is transient notification only

- Status: accepted
- Date: 2026-11-22

## Context

WebSocket clients need low-latency updates when events commit, but Redis data
is volatile and must never be required to know what occurred.

## Decision

Redis pub/sub carries wake-up/fan-out notifications derived from the
PostgreSQL `event_outbox` table. A publisher reads committed outbox rows and
publishes them; a Redis outage only delays notifications — events are replayed
from the outbox when Redis recovers. Clients reconnect using
`GET /api/v1/runs/{id}/events?afterSequence=N` and deduplicate by the
monotonic per-run sequence number.

## Consequences

- Redis can be flushed or restarted without data loss.
- At-least-once delivery to subscribers; deduplication is the client's job,
  aided by sequence numbers.
- No queue or lock state lives in Redis.
