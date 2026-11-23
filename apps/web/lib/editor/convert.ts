/**
 * Conversion between the React Flow editor state and the canonical
 * contract graph (schemaVersion "1"). Used by validation, autosave,
 * import/export and the persistence round-trip.
 */
import {
  GRAPH_SCHEMA_VERSION,
  type GraphNodeKind,
  type WorkflowGraph,
} from "@/lib/contracts/graph";
import type { FlowEdge, FlowNode } from "./store";

export function toWorkflowGraph(nodes: FlowNode[], edges: FlowEdge[]): WorkflowGraph {
  return {
    schemaVersion: GRAPH_SCHEMA_VERSION,
    nodes: nodes.map((n) => ({
      id: n.id,
      kind: (n.data.kind ?? n.type) as GraphNodeKind,
      label: n.data.label,
      position: { x: n.position.x, y: n.position.y },
      ...(n.data.config ? { config: n.data.config } : {}),
    })),
    edges: edges.map((e) => ({
      id: e.id,
      source: e.source,
      sourceHandle: e.sourceHandle ?? null,
      target: e.target,
      targetHandle: e.targetHandle ?? null,
      ...(typeof e.label === "string" ? { label: e.label } : {}),
    })),
  };
}

export function fromWorkflowGraph(graph: WorkflowGraph): {
  nodes: FlowNode[];
  edges: FlowEdge[];
} {
  const nodes: FlowNode[] = graph.nodes.map((n, i) => ({
    id: n.id,
    type: n.kind,
    position: n.position ?? { x: (i % 4) * 220, y: Math.floor(i / 4) * 140 },
    data: {
      label: n.label,
      kind: n.kind,
      ...(n.config ? { config: n.config } : {}),
    },
    ...(n.kind === "start" || n.kind === "end" ? { deletable: false } : {}),
  }));
  const edges: FlowEdge[] = graph.edges.map((e) => ({
    id: e.id,
    source: e.source,
    sourceHandle: e.sourceHandle ?? undefined,
    target: e.target,
    targetHandle: e.targetHandle ?? undefined,
    ...(e.label ? { label: e.label } : {}),
  }));
  return { nodes, edges };
}

/** Recomputes per-kind counters from loaded nodes so new IDs never collide. */
export function countersFromNodes(nodes: FlowNode[]): Record<string, number> {
  const counters: Record<string, number> = {};
  for (const node of nodes) {
    const match = /^([a-z]+)-(\d+)$/.exec(node.id);
    if (!match) continue;
    const [, kind, n] = match;
    counters[kind] = Math.max(counters[kind] ?? 0, Number(n));
  }
  return counters;
}
