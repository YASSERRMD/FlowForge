# ADR 0002: DAG-only workflow graphs in v1

- Status: accepted
- Date: 2026-11-22

## Context

Workflow graphs could support arbitrary loops, but loops complicate join
semantics, termination guarantees, event ordering and recovery.

## Decision

Version 1 of the graph schema accepts **directed acyclic graphs only**. Both
the TypeScript client validator and the Go server validator reject cycles.
Loops/repetition are expressed as step retry policy on a node, never as graph
edges that point backwards.

## Consequences

- Topological scheduling is always possible; joins cannot deadlock on a cycle.
- Retry history stays per-node and per-attempt instead of unbounded revisits.
- A future schema version may add bounded loop constructs; `schemaVersion`
  gates that migration.
