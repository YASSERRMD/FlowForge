/**
 * Graph import bounds: size caps and clear, user-facing errors for every
 * failure mode. The server enforces the same limits (256 KiB body cap,
 * 200/400 node/edge limits in graph.Validate), so a document that passes
 * here also passes there.
 */
import type { WorkflowGraph } from "@/lib/contracts/graph";
import { MAX_GRAPH_EDGES, MAX_GRAPH_NODES } from "@/lib/contracts/graph";
import { validateGraph, type GraphIssue } from "./validate";

/** Matches the API's maxWorkflowBody. */
export const MAX_IMPORT_BYTES = 256 * 1024;

export type ImportResult =
  | { ok: true; graph: WorkflowGraph; warnings: GraphIssue[] }
  | { ok: false; error: string; issues?: GraphIssue[] };

const encoder = new TextEncoder();

export function parseGraphDocument(text: string): ImportResult {
  if (encoder.encode(text).length > MAX_IMPORT_BYTES) {
    return {
      ok: false,
      error: `File is too large (limit ${MAX_IMPORT_BYTES / 1024} KiB). Split the workflow into smaller graphs.`,
    };
  }

  let parsed: unknown;
  try {
    parsed = JSON.parse(text);
  } catch (err) {
    return {
      ok: false,
      error: `Not valid JSON: ${err instanceof Error ? err.message : "parse error"}.`,
    };
  }

  if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) {
    return { ok: false, error: "A workflow graph must be a JSON object." };
  }

  const candidate = parsed as Partial<WorkflowGraph>;
  if (candidate.schemaVersion !== "1") {
    return {
      ok: false,
      error: `Unsupported schemaVersion "${String(candidate.schemaVersion)}"; only "1" is supported. See docs/contracts/graph.md.`,
    };
  }
  if (!Array.isArray(candidate.nodes) || !Array.isArray(candidate.edges)) {
    return { ok: false, error: 'A graph document needs "nodes" and "edges" arrays.' };
  }
  if (candidate.nodes.length > MAX_GRAPH_NODES) {
    return {
      ok: false,
      error: `Too many nodes (${candidate.nodes.length}); the limit is ${MAX_GRAPH_NODES}.`,
    };
  }
  if (candidate.edges.length > MAX_GRAPH_EDGES) {
    return {
      ok: false,
      error: `Too many edges (${candidate.edges.length}); the limit is ${MAX_GRAPH_EDGES}.`,
    };
  }

  const graph = candidate as WorkflowGraph;
  const issues = validateGraph(graph);
  const errors = issues.filter((i) => i.severity === "error");
  if (errors.length > 0) {
    return {
      ok: false,
      error: `The graph has ${errors.length} structural error${errors.length === 1 ? "" : "s"}; fix them before importing.`,
      issues: errors,
    };
  }
  return { ok: true, graph, warnings: issues.filter((i) => i.severity === "warning") };
}

/** Serializes a graph for export with stable, human-readable formatting. */
export function exportGraphDocument(graph: WorkflowGraph): string {
  return JSON.stringify(graph, null, 2) + "\n";
}
