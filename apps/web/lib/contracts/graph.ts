/**
 * TypeScript mirror of packages/contracts/graph-schema.json (schemaVersion "1").
 * Keep in sync with the JSON schema and with internal/graph/types.go.
 * See docs/contracts/graph.md for semantics and the migration policy.
 */

export const GRAPH_SCHEMA_VERSION = "1" as const;

export type GraphNodeKind =
  | "start"
  | "end"
  | "action"
  | "delay"
  | "condition"
  | "fork"
  | "join";

export interface GraphPosition {
  x: number;
  y: number;
}

export interface GraphNodeConfig {
  /** action: registered action type identifier. */
  actionType?: string;
  /** action: free-form parameters passed to the action. */
  params?: Record<string, unknown>;
  /** delay: wait duration in milliseconds (positive). */
  durationMs?: number;
  /** condition: expression in the condition grammar. */
  expression?: string;
  /** fork/join: number of parallel branches (>= 2). */
  branchCount?: number;
  /** Unknown keys from newer writers are preserved verbatim. */
  [key: string]: unknown;
}

export interface GraphNode {
  id: string;
  kind: GraphNodeKind;
  label: string;
  position?: GraphPosition;
  config?: GraphNodeConfig;
}

export interface GraphEdge {
  id: string;
  source: string;
  sourceHandle?: string | null;
  target: string;
  targetHandle?: string | null;
  label?: string;
}

export interface WorkflowGraph {
  schemaVersion: typeof GRAPH_SCHEMA_VERSION;
  nodes: GraphNode[];
  edges: GraphEdge[];
}

export const MAX_GRAPH_NODES = 200;
export const MAX_GRAPH_EDGES = 400;

/** Port rules per node kind: which source handles may originate edges. */
export const SOURCE_HANDLES: Record<GraphNodeKind, (config?: GraphNodeConfig) => string[]> = {
  start: () => ["default"],
  end: () => [],
  action: () => ["default"],
  delay: () => ["default"],
  condition: () => ["true", "false"],
  fork: (config) => {
    const n = typeof config?.branchCount === "number" && config.branchCount >= 2 ? config.branchCount : 2;
    return Array.from({ length: n }, (_, i) => `branch-${i + 1}`);
  },
  join: () => ["default"],
};

/** Whether the kind may accept incoming edges. */
export function acceptsIncoming(kind: GraphNodeKind): boolean {
  return kind !== "start";
}

/** Whether the kind may originate outgoing edges. */
export function hasOutgoing(kind: GraphNodeKind): boolean {
  return kind !== "end";
}
