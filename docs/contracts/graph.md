# Workflow Graph Contract

The canonical persisted form of a FlowForge workflow is the graph document
defined by [`packages/contracts/graph-schema.json`](../../packages/contracts/graph-schema.json).

```json
{
  "schemaVersion": "1",
  "nodes": [{ "id": "start-1", "kind": "start", "label": "Start" }],
  "edges": []
}
```

## Node kinds

| Kind        | Semantics                                            | Handles |
| ----------- | ---------------------------------------------------- | ------- |
| `start`     | Entry point. Exactly one required. No incoming edges.| one source |
| `end`       | Terminal. At least one required. No outgoing edges.  | one target |
| `action`    | Runs a registered action.                            | one target, one source |
| `delay`     | Waits for a duration.                                | one target, one source |
| `condition` | Branches on a typed expression.                      | one target, sources `true`/`false` |
| `fork`      | Fans out to parallel branches.                       | one target, sources `branch-N` |
| `join`      | Joins parallel branches.                             | multiple targets, one source |

## Config keys per kind

`config` is an open object per node kind. Keys reserved by the platform:

- `action`: `actionType` (string), `params` (object)
- `delay`: `durationMs` (positive integer)
- `condition`: `expression` (string in the condition grammar, see
  [conditions.md](conditions.md))
- `fork`/`join`: `branchCount` (integer ≥ 2)

Unknown config keys MUST be preserved verbatim on round-trip (write what you
read) so older clients never destroy data written by newer ones.

## Versioning and migration policy

- `schemaVersion` is a string. `"1"` is the only version currently emitted.
- Readers MUST reject documents with any other `schemaVersion` with a clear
  error; there is no silent best-effort parsing across versions.
- When a `"2"` is introduced, a deterministic `1 → 2` migration will ship in
  the same release, applied on read and re-persisted on the next save. Migrations
  are pure functions with unit tests covering every node/edge shape.
- Within one version, the document is canonical: key order in serialized JSON
  is fixed by the writer, and a load→save round-trip with no edits MUST be
  byte-stable.

## Structural invariants (enforced client- and server-side)

- The graph is a DAG (no cycles, no self-loops).
- Exactly one `start`, at least one `end`; every node is reachable from `start`.
- No duplicate node or edge IDs; no duplicate edges on the same source/handle
  pair.
- `condition` edges only leave through handles `true`/`false`; `fork` edges
  through `branch-N`; `end` has no outgoing edges; `start` has no incoming.
- Limits: ≤ 200 nodes, ≤ 400 edges, IDs/labels length-bounded by the schema.
