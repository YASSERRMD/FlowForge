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

export type FlowNode = Node<FlowNodeData>;
export type FlowEdge = Edge;

interface EditorState {
  nodes: FlowNode[];
  edges: FlowEdge[];
  counters: Record<NodeKind, number>;
  onNodesChange: (changes: NodeChangeset<FlowNode>) => void;
  onEdgesChange: (changes: EdgeChangeset<FlowEdge>) => void;
  onConnect: (connection: Connection) => void;
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

  onConnect: (connection) => {
    const { edges } = get();
    const edge: FlowEdge = {
      ...connection,
      id: `e-${connection.source}-${connection.sourceHandle ?? "out"}-${connection.target}-${connection.targetHandle ?? "in"}`,
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
