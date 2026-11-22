# Phase 05 — Visual graph canvas and editing interactions

Branch: `phase/05-visual-graph-designer` (from merged `origin/main` after PR #4)

## Task journal

| # | Task | Commit | Evidence |
|---|------|--------|----------|
| 05.01 | Mount react flow canvas | `4f416f1` | @xyflow/react v13 (changeset API), `ReactFlowProvider`, fit-to-view, route `/workflows/[id]` renders the editor (dynamic route in build output) |
| 05.02 | Create drag node palette | `9b3580a` | Keyboard-usable palette buttons (Enter/click adds at canvas center) + HTML5 drag/drop with `screenToFlowPosition`; deterministic `${kind}-${n}` IDs proven by unit tests |
| 05.03 | Render start end nodes | `82bc7b4` | Distinct success/error chrome; start = source-only, end = target-only; both non-deletable |
| 05.04 | Render action and delay nodes | `cd08376` | Icons, double-click inline label editing (Enter/blur commit, Esc cancel), top/bottom handles, sr-only kind labels |
| 05.05 | Render condition node | `cafc392` | Labeled `true`/`false` source handles at 25%/75%; store stamps branch labels + `data.branch` on edges; unit tests |
| 05.06 | Render parallel fork join nodes | `ecfa0e8` | Fork (3 labeled outputs) / Join (3 inputs); fork edges animated for traceability |
| 05.07 | Implement edge editing | `3e6f24a` | `isValidConnection` constraints: no self-loops, no edges into start / out of end, no duplicates, **no cycles (DAG, ADR 0002)**; reconnect with same validation; 7 unit tests + cycle DFS tests |
| 05.08 | Add minimap and canvas tools | `309f913` | Controls, pannable/zoomable MiniMap, snap-to-grid 16px, drag-selection, multi-select keys, zoom bounds |
| 05.09 | Add undo redo history | `f703d29` | Snapshot history (limit 50) over add/connect/reconnect/label/drag-end/delete; counters restored on undo (IDs stay deterministic); 9 unit tests |
| 05.10 | Add keyboard copy paste | `1ff3138` | Copy/paste/duplicate/select-all/delete/undo/redo shortcuts; text-field guard (inputs, textarea, contentEditable never hijacked); ID-remapped pastes; 4 unit tests |
| 05.11 | Test editor interactions | `5382ea1` (+ `0a7cee7`) | Palette component tests (4) + **5 Playwright scenarios against the production build**: mount, deterministic labels, delete+undo keyboard, duplicate, start/end non-deletable — all passing locally; new `e2e` CI job |

## Phase gate results

- `pnpm lint` / `tsc --noEmit` — clean
- `pnpm test` — 70/70 vitest (8 files)
- `pnpm build` — pass; `/workflows/[id]` dynamic route renders
- `pnpm test:e2e` — 5/5 Playwright (Chromium headless, production bundle)
- Go untouched: `go vet`/`test`/`race`/`build` re-verified at gate (with integration DB)
- `git diff --check` clean; 12 commits `phase 05: *`; author=committer YASSERRMD
- CI gained `e2e` job (chromium + build + playwright, artifact on failure)

## Design decisions

- Graph state lives in a zustand store with serializable snapshots — undo/redo, clipboard, and (later) persistence all share the same shape.
- DAG validity is enforced client-side at connection time (`lib/editor/graph.ts`); the same function backs live drag feedback and reconnect.
- Editor canvas state is intentionally not persisted yet — the page says so; API persistence lands with the workflows backend phase.
