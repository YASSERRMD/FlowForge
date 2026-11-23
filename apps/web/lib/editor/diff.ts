/**
 * Semantic diff between two graph documents: node/edge additions, removals
 * and meaningful changes (label, kind, config, position), keyed by stable
 * node/edge IDs.
 */
import type { GraphEdge, GraphNode, WorkflowGraph } from "@/lib/contracts/graph";

export interface NodeChange {
  id: string;
  label: string;
  changes: string[];
}

export interface GraphDiff {
  nodesAdded: GraphNode[];
  nodesRemoved: GraphNode[];
  nodesChanged: NodeChange[];
  edgesAdded: GraphEdge[];
  edgesRemoved: GraphEdge[];
  identical: boolean;
}

function configFingerprint(config: GraphNode["config"]): string {
  if (!config) return "";
  const keys = Object.keys(config).sort();
  return keys.map((k) => `${k}=${JSON.stringify(config[k])}`).join("&");
}

export function diffGraphs(before: WorkflowGraph, after: WorkflowGraph): GraphDiff {
  const beforeNodes = new Map(before.nodes.map((n) => [n.id, n]));
  const afterNodes = new Map(after.nodes.map((n) => [n.id, n]));

  const nodesAdded: GraphNode[] = [];
  const nodesRemoved: GraphNode[] = [];
  const nodesChanged: NodeChange[] = [];

  for (const node of after.nodes) {
    const prev = beforeNodes.get(node.id);
    if (!prev) {
      nodesAdded.push(node);
      continue;
    }
    const changes: string[] = [];
    if (prev.kind !== node.kind) changes.push(`kind: ${prev.kind} → ${node.kind}`);
    if (prev.label !== node.label) changes.push(`label: "${prev.label}" → "${node.label}"`);
    const prevCfg = configFingerprint(prev.config);
    const nextCfg = configFingerprint(node.config);
    if (prevCfg !== nextCfg) changes.push("configuration changed");
    if (
      prev.position?.x !== node.position?.x ||
      prev.position?.y !== node.position?.y
    ) {
      changes.push("moved on canvas");
    }
    if (changes.length > 0) {
      nodesChanged.push({ id: node.id, label: node.label, changes });
    }
  }
  for (const node of before.nodes) {
    if (!afterNodes.has(node.id)) nodesRemoved.push(node);
  }

  const beforeEdges = new Map(before.edges.map((e) => [e.id, e]));
  const afterEdges = new Map(after.edges.map((e) => [e.id, e]));
  const edgesAdded = after.edges.filter((e) => !beforeEdges.has(e.id));
  const edgesRemoved = before.edges.filter((e) => !afterEdges.has(e.id));

  const identical =
    nodesAdded.length === 0 &&
    nodesRemoved.length === 0 &&
    nodesChanged.length === 0 &&
    edgesAdded.length === 0 &&
    edgesRemoved.length === 0;

  return { nodesAdded, nodesRemoved, nodesChanged, edgesAdded, edgesRemoved, identical };
}

/** One-line human summary, used in the comparison header. */
export function summarizeDiff(diff: GraphDiff): string {
  if (diff.identical) return "No differences";
  const parts: string[] = [];
  if (diff.nodesAdded.length) parts.push(`${diff.nodesAdded.length} node${diff.nodesAdded.length > 1 ? "s" : ""} added`);
  if (diff.nodesRemoved.length) parts.push(`${diff.nodesRemoved.length} node${diff.nodesRemoved.length > 1 ? "s" : ""} removed`);
  if (diff.nodesChanged.length) parts.push(`${diff.nodesChanged.length} node${diff.nodesChanged.length > 1 ? "s" : ""} changed`);
  if (diff.edgesAdded.length) parts.push(`${diff.edgesAdded.length} edge${diff.edgesAdded.length > 1 ? "s" : ""} added`);
  if (diff.edgesRemoved.length) parts.push(`${diff.edgesRemoved.length} edge${diff.edgesRemoved.length > 1 ? "s" : ""} removed`);
  return parts.join(", ");
}
