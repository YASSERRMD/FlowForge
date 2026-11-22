import type { Connection, Edge, Node } from "@xyflow/react";

/** Returns true when adding source→target would create a cycle. */
export function wouldCreateCycle(
  edges: Pick<Edge, "source" | "target">[],
  source: string,
  target: string,
): boolean {
  // DFS from target: can we reach source?
  const adjacency = new Map<string, string[]>();
  for (const e of edges) {
    const list = adjacency.get(e.source) ?? [];
    list.push(e.target);
    adjacency.set(e.source, list);
  }
  const stack = [target];
  const visited = new Set<string>();
  while (stack.length > 0) {
    const current = stack.pop()!;
    if (current === source) return true;
    if (visited.has(current)) continue;
    visited.add(current);
    for (const next of adjacency.get(current) ?? []) stack.push(next);
  }
  return false;
}

/**
 * Client-side DAG constraints enforced before any edge is created:
 * - no self-loops
 * - start accepts no incoming edges; end originates none
 * - no duplicate edges on the same source/target handles
 * - the graph must stay acyclic (ADR 0002: DAG-only v1)
 */
export function isValidConnection(
  connection: Connection | Edge,
  nodes: Pick<Node, "id" | "type">[],
  edges: Pick<Edge, "source" | "sourceHandle" | "target" | "targetHandle" | "id">[],
  ignoreEdgeId?: string,
): boolean {
  const { source, target } = connection;
  if (!source || !target || source === target) return false;

  const sourceNode = nodes.find((n) => n.id === source);
  const targetNode = nodes.find((n) => n.id === target);
  if (!sourceNode || !targetNode) return false;
  if (sourceNode.type === "end" || targetNode.type === "start") return false;

  const duplicate = edges.some(
    (e) =>
      e.id !== ignoreEdgeId &&
      e.source === source &&
      e.target === target &&
      (e.sourceHandle ?? null) === (connection.sourceHandle ?? null) &&
      (e.targetHandle ?? null) === (connection.targetHandle ?? null),
  );
  if (duplicate) return false;

  const remaining = edges.filter((e) => e.id !== ignoreEdgeId);
  return !wouldCreateCycle(remaining, source, target);
}
