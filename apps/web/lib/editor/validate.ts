/**
 * Client-side graph validation over the canonical contract shape.
 * Mirrors the Go validator in internal/graph (06.04); keep rules in sync.
 */
import {
  MAX_GRAPH_EDGES,
  MAX_GRAPH_NODES,
  SOURCE_HANDLES,
  acceptsIncoming,
  hasOutgoing,
  type GraphNode,
  type WorkflowGraph,
} from "@/lib/contracts/graph";
import { validateConfig } from "./config-schema";

export type IssueSeverity = "error" | "warning";

export interface GraphIssue {
  severity: IssueSeverity;
  code:
    | "schema_version"
    | "size_limit"
    | "duplicate_node_id"
    | "duplicate_edge_id"
    | "missing_start"
    | "multiple_start"
    | "missing_end"
    | "unreachable_node"
    | "orphan_node"
    | "dangling_edge"
    | "self_loop"
    | "duplicate_edge"
    | "invalid_source_handle"
    | "invalid_target_kind"
    | "cycle"
    | "config";
  message: string;
  /** Node or edge ID the issue points at (drives click-to-focus). */
  nodeId?: string;
  edgeId?: string;
}

const ID_PATTERN = /^[a-zA-Z0-9_-]+$/;

export function validateGraph(graph: WorkflowGraph): GraphIssue[] {
  const issues: GraphIssue[] = [];

  if (graph.schemaVersion !== "1") {
    issues.push({
      severity: "error",
      code: "schema_version",
      message: `Unsupported schemaVersion "${String(graph.schemaVersion)}"; only "1" is supported.`,
    });
  }

  if (graph.nodes.length > MAX_GRAPH_NODES) {
    issues.push({
      severity: "error",
      code: "size_limit",
      message: `Graph has ${graph.nodes.length} nodes; the limit is ${MAX_GRAPH_NODES}.`,
    });
  }
  if (graph.edges.length > MAX_GRAPH_EDGES) {
    issues.push({
      severity: "error",
      code: "size_limit",
      message: `Graph has ${graph.edges.length} edges; the limit is ${MAX_GRAPH_EDGES}.`,
    });
  }

  const nodeById = new Map<string, GraphNode>();
  for (const node of graph.nodes) {
    if (!ID_PATTERN.test(node.id)) {
      issues.push({
        severity: "error",
        code: "duplicate_node_id",
        nodeId: node.id,
        message: `Node ID "${node.id}" is empty or contains invalid characters.`,
      });
    }
    if (nodeById.has(node.id)) {
      issues.push({
        severity: "error",
        code: "duplicate_node_id",
        nodeId: node.id,
        message: `Duplicate node ID "${node.id}".`,
      });
      continue;
    }
    nodeById.set(node.id, node);
  }

  const starts = graph.nodes.filter((n) => n.kind === "start");
  const ends = graph.nodes.filter((n) => n.kind === "end");
  if (starts.length === 0) {
    issues.push({
      severity: "error",
      code: "missing_start",
      message: "Graph must contain exactly one start node.",
    });
  } else if (starts.length > 1) {
    for (const s of starts) {
      issues.push({
        severity: "error",
        code: "multiple_start",
        nodeId: s.id,
        message: "Graph must contain exactly one start node.",
      });
    }
  }
  if (ends.length === 0) {
    issues.push({
      severity: "error",
      code: "missing_end",
      message: "Graph must contain at least one end node.",
    });
  }

  const edgeIds = new Set<string>();
  const edgeKeys = new Set<string>();
  const incoming = new Map<string, number>();
  const outgoing = new Map<string, GraphNode[]>();
  for (const edge of graph.edges) {
    if (edgeIds.has(edge.id)) {
      issues.push({
        severity: "error",
        code: "duplicate_edge_id",
        edgeId: edge.id,
        message: `Duplicate edge ID "${edge.id}".`,
      });
    }
    edgeIds.add(edge.id);

    if (edge.source === edge.target) {
      issues.push({
        severity: "error",
        code: "self_loop",
        edgeId: edge.id,
        nodeId: edge.source,
        message: `Edge "${edge.id}" connects node "${edge.source}" to itself.`,
      });
    }

    const source = nodeById.get(edge.source);
    const target = nodeById.get(edge.target);
    if (!source || !target) {
      issues.push({
        severity: "error",
        code: "dangling_edge",
        edgeId: edge.id,
        message: `Edge "${edge.id}" references a missing node (${!source ? edge.source : edge.target}).`,
      });
      continue;
    }

    const key = `${edge.source}${edge.sourceHandle ?? ""}→${edge.target}${edge.targetHandle ?? ""}`;
    if (edgeKeys.has(key)) {
      issues.push({
        severity: "error",
        code: "duplicate_edge",
        edgeId: edge.id,
        message: `Duplicate edge from "${edge.source}" to "${edge.target}".`,
      });
    }
    edgeKeys.add(key);

    if (!hasOutgoing(source.kind)) {
      issues.push({
        severity: "error",
        code: "invalid_source_handle",
        edgeId: edge.id,
        nodeId: source.id,
        message: `Node "${source.id}" (${source.kind}) cannot have outgoing edges.`,
      });
    } else {
      const handle = edge.sourceHandle ?? "default";
      const allowed = SOURCE_HANDLES[source.kind](source.config);
      if (!allowed.includes(handle)) {
        issues.push({
          severity: "error",
          code: "invalid_source_handle",
          edgeId: edge.id,
          nodeId: source.id,
          message: `Edge "${edge.id}" uses unknown source handle "${handle}" on node "${source.id}".`,
        });
      }
    }

    if (!acceptsIncoming(target.kind)) {
      issues.push({
        severity: "error",
        code: "invalid_target_kind",
        edgeId: edge.id,
        nodeId: target.id,
        message: `Node "${target.id}" (${target.kind}) cannot have incoming edges.`,
      });
    }

    incoming.set(target.id, (incoming.get(target.id) ?? 0) + 1);
    const list = outgoing.get(source.id) ?? [];
    list.push(target);
    outgoing.set(source.id, list);
  }

  // Reachability from start + cycle detection via DFS coloring.
  const adjacency = new Map<string, string[]>();
  for (const edge of graph.edges) {
    const list = adjacency.get(edge.source) ?? [];
    list.push(edge.target);
    adjacency.set(edge.source, list);
  }

  const color = new Map<string, 1 | 2>(); // 1 = in stack, 2 = done
  let cyclic = false;
  const dfs = (id: string) => {
    if (cyclic) return;
    color.set(id, 1);
    for (const next of adjacency.get(id) ?? []) {
      if (color.get(next) === 1) {
        cyclic = true;
        issues.push({
          severity: "error",
          code: "cycle",
          nodeId: next,
          message: `Graph contains a cycle involving node "${next}".`,
        });
        return;
      }
      if (!color.has(next)) dfs(next);
    }
    color.set(id, 2);
  };
  for (const node of graph.nodes) {
    if (!color.has(node.id)) dfs(node.id);
  }

  if (starts.length === 1) {
    const reachable = new Set<string>();
    const stack = [starts[0].id];
    while (stack.length > 0) {
      const id = stack.pop()!;
      if (reachable.has(id)) continue;
      reachable.add(id);
      for (const next of adjacency.get(id) ?? []) stack.push(next);
    }
    for (const node of graph.nodes) {
      if (!reachable.has(node.id)) {
        issues.push({
          severity: "error",
          code: "unreachable_node",
          nodeId: node.id,
          message: `Node "${node.id}" is not reachable from the start node.`,
        });
      }
    }
  }

  // Orphans: no edges at all and not the start/end of a trivial graph.
  if (graph.nodes.length > 2) {
    for (const node of graph.nodes) {
      if (node.kind === "start" || node.kind === "end") continue;
      const hasIn = (incoming.get(node.id) ?? 0) > 0;
      const hasOut = (outgoing.get(node.id) ?? []).length > 0;
      if (!hasIn && !hasOut) {
        issues.push({
          severity: "warning",
          code: "orphan_node",
          nodeId: node.id,
          message: `Node "${node.id}" (${node.label}) is not connected to anything.`,
        });
      }
    }
  }

  // Per-node config issues.
  for (const node of graph.nodes) {
    for (const issue of validateConfig(node.kind, node.config)) {
      issues.push({
        severity: "error",
        code: "config",
        nodeId: node.id,
        message: `${node.label}: ${issue.message}`,
      });
    }
  }

  return issues;
}
