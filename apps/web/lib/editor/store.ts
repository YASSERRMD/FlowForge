"use client";

import { create } from "zustand";
import {
  type Connection,
  type Edge,
  type EdgeChangeset,
  type Node,
  type NodeChangeset,
} from "@xyflow/react";

import type { FlowNodeData, NodeKind } from "./types";
import { NODE_KIND_LABELS } from "./types";
import { isValidConnection } from "./graph";

export type FlowNode = Node<FlowNodeData>;
export type FlowEdge = Edge;

interface EditorState {
  nodes: FlowNode[];
  edges: FlowEdge[];
  counters: Record<NodeKind, number>;
  onNodesChange: (changes: NodeChangeset<FlowNode>) => void;
  onEdgesChange: (changes: EdgeChangeset<FlowEdge>) => void;
  onConnect: (connection: Connection) => void;
  onReconnect: (oldEdge: FlowEdge, connection: Connection) => void;
  canConnect: (connection: Connection | FlowEdge, ignoreEdgeId?: string) => boolean;
  addNode: (kind: NodeKind, position: { x: number; y: number }) => FlowNode;
  updateNodeLabel: (id: string, label: string) => void;
  setNodes: (nodes: FlowNode[]) => void;
  setEdges: (edges: FlowEdge[]) => void;
}

const initialNodes: FlowNode[] = [
  {
    id: "start",
    type: "start",
    position: { x: 0, y: 0 },
    data: { label: "Start", kind: "start" },
    deletable: false,
  },
  {
    id: "end",
    type: "end",
    position: { x: 0, y: 300 },
    data: { label: "End", kind: "end" },
    deletable: false,
  },
];

/** Deterministic node IDs: `${kind}-${n}` per kind counter. */
function nextId(counters: Record<NodeKind, number>, kind: NodeKind) {
  const n = (counters[kind] ?? 0) + 1;
  return { id: `${kind}-${n}`, counters: { ...counters, [kind]: n } };
}

export const useEditorStore = create<EditorState>((set, get) => ({
  nodes: initialNodes,
  edges: [],
  counters: { start: 1, end: 1 } as Record<NodeKind, number>,

  onNodesChange: (changes) =>
    set({ nodes: changes.applyTo(get().nodes) }),

  onEdgesChange: (changes) =>
    set({ edges: changes.applyTo(get().edges) }),

  canConnect: (connection, ignoreEdgeId) => {
    const { nodes, edges } = get();
    return isValidConnection(connection, nodes, edges, ignoreEdgeId);
  },

  onReconnect: (oldEdge, connection) => {
    const { nodes, edges } = get();
    if (!isValidConnection(connection, nodes, edges, oldEdge.id)) return;
    const updated: FlowEdge = {
      ...oldEdge,
      ...connection,
      id: `e-${connection.source}-${connection.sourceHandle ?? "out"}-${connection.target}-${connection.targetHandle ?? "in"}`,
    };
    set({ edges: edges.map((e) => (e.id === oldEdge.id ? updated : e)) });
  },

  onConnect: (connection) => {
    const { edges, nodes } = get();
    if (!isValidConnection(connection, nodes, edges)) return;
    const source = nodes.find((n) => n.id === connection.source);
    // Branch edges carry their outcome as a visible label.
    const branchLabel =
      source?.type === "condition" &&
      (connection.sourceHandle === "true" || connection.sourceHandle === "false")
        ? connection.sourceHandle
        : undefined;
    // Fork edges are animated so parallel paths stay visually traceable.
    const fromFork = source?.type === "fork";
    const edge: FlowEdge = {
      ...connection,
      id: `e-${connection.source}-${connection.sourceHandle ?? "out"}-${connection.target}-${connection.targetHandle ?? "in"}`,
      label: branchLabel ?? (fromFork ? (connection.sourceHandle ?? undefined) : undefined),
      animated: fromFork,
      data: branchLabel ? { branch: branchLabel } : undefined,
    };
    set({ edges: [...edges, edge] });
  },

  addNode: (kind, position) => {
    const { counters, nodes } = get();
    const { id, counters: next } = nextId(counters, kind);
    const node: FlowNode = {
      id,
      type: kind,
      position,
      data: { label: `${NODE_KIND_LABELS[kind]} ${next[kind]}`, kind },
    };
    set({ nodes: [...nodes, node], counters: next });
    return node;
  },

  updateNodeLabel: (id, label) =>
    set({
      nodes: get().nodes.map((n) =>
        n.id === id ? { ...n, data: { ...n.data, label } } : n,
      ),
    }),

  setNodes: (nodes) => set({ nodes }),
  setEdges: (edges) => set({ edges }),
}));
