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

interface Snapshot {
  nodes: FlowNode[];
  edges: FlowEdge[];
  counters: Record<NodeKind, number>;
}

const HISTORY_LIMIT = 50;

interface EditorState {
  nodes: FlowNode[];
  edges: FlowEdge[];
  counters: Record<NodeKind, number>;
  past: Snapshot[];
  future: Snapshot[];
  clipboard: { nodes: FlowNode[]; edges: FlowEdge[] } | null;
  onNodesChange: (changes: NodeChangeset<FlowNode>) => void;
  onEdgesChange: (changes: EdgeChangeset<FlowEdge>) => void;
  onConnect: (connection: Connection) => void;
  onReconnect: (oldEdge: FlowEdge, connection: Connection) => void;
  canConnect: (connection: Connection | FlowEdge, ignoreEdgeId?: string) => boolean;
  addNode: (kind: NodeKind, position: { x: number; y: number }) => FlowNode;
  updateNodeLabel: (id: string, label: string) => void;
  updateNodeConfig: (id: string, patch: Record<string, unknown>) => void;
  deleteSelection: () => void;
  copySelection: () => void;
  pasteClipboard: () => void;
  duplicateSelection: () => void;
  selectAll: () => void;
  undo: () => void;
  redo: () => void;
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

export const useEditorStore = create<EditorState>((set, get) => {
  /** Pushes the current state onto the undo stack and clears redo. */
  const checkpoint = () => {
    const { nodes, edges, counters, past } = get();
    const snapshot: Snapshot = {
      nodes: structuredClone(nodes),
      edges: structuredClone(edges),
      counters: { ...counters },
    };
    set({ past: [...past.slice(-(HISTORY_LIMIT - 1)), snapshot], future: [] });
  };

  /** Inserts cloned nodes/edges with fresh deterministic IDs, offset and
   * selected. Node kinds drive the new IDs via the shared counter. */
  const insertCopies = (
    getState: () => EditorState,
    setState: (partial: Partial<EditorState>) => void,
    srcNodes: FlowNode[],
    srcEdges: FlowEdge[],
  ) => {
    let { counters } = getState();
    const { nodes, edges } = getState();
    const idMap = new Map<string, string>();
    const newNodes: FlowNode[] = [];

    for (const node of srcNodes) {
      const kind = (node.data.kind ?? node.type) as NodeKind;
      const { id, counters: next } = nextId(counters, kind);
      counters = next;
      idMap.set(node.id, id);
      newNodes.push({
        ...node,
        id,
        position: { x: node.position.x + 32, y: node.position.y + 32 },
        selected: true,
      });
    }

    const newEdges: FlowEdge[] = [];
    for (const edge of srcEdges) {
      const source = idMap.get(edge.source);
      const target = idMap.get(edge.target);
      if (!source || !target) continue;
      newEdges.push({
        ...edge,
        id: `e-${source}-${edge.sourceHandle ?? "out"}-${target}-${edge.targetHandle ?? "in"}`,
        source,
        target,
        selected: true,
      });
    }

    setState({
      nodes: [...nodes.map((n) => ({ ...n, selected: false })), ...newNodes],
      edges: [...edges.map((e) => ({ ...e, selected: false })), ...newEdges],
      counters,
    });
  };

  return {
    nodes: initialNodes,
    edges: [],
    counters: { start: 1, end: 1 } as Record<NodeKind, number>,
    past: [],
    future: [],
    clipboard: null,

    onNodesChange: (changes) => {
      // Record deletions and completed drags; skip selection/hover noise
      // and in-progress drag frames.
      const arr = changes.toArray();
      const hasRemoval = arr.some((c) => c.type === "remove");
      const dragEnded = arr.some(
        (c) => c.type === "position" && c.dragging === false,
      );
      const deletableRemoval = arr.some(
        (c) =>
          c.type === "remove" &&
          get().nodes.find((n) => n.id === c.id)?.deletable !== false,
      );
      if (hasRemoval && deletableRemoval) checkpoint();
      else if (dragEnded && !hasRemoval) checkpoint();
      set({ nodes: changes.applyTo(get().nodes) });
    },

    onEdgesChange: (changes) => {
      const hasRemoval = changes.toArray().some((c) => c.type === "remove");
      if (hasRemoval) checkpoint();
      set({ edges: changes.applyTo(get().edges) });
    },

    canConnect: (connection, ignoreEdgeId) => {
      const { nodes, edges } = get();
      return isValidConnection(connection, nodes, edges, ignoreEdgeId);
    },

    onReconnect: (oldEdge, connection) => {
      const { nodes, edges } = get();
      if (!isValidConnection(connection, nodes, edges, oldEdge.id)) return;
      checkpoint();
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
      checkpoint();
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
        label:
          branchLabel ?? (fromFork ? (connection.sourceHandle ?? undefined) : undefined),
        animated: fromFork,
        data: branchLabel ? { branch: branchLabel } : undefined,
      };
      set({ edges: [...edges, edge] });
    },

    addNode: (kind, position) => {
      const { counters, nodes } = get();
      checkpoint();
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

    updateNodeLabel: (id, label) => {
      const target = get().nodes.find((n) => n.id === id);
      if (!target || target.data.label === label) return;
      checkpoint();
      set({
        nodes: get().nodes.map((n) =>
          n.id === id ? { ...n, data: { ...n.data, label } } : n,
        ),
      });
    },

    updateNodeConfig: (id, patch) => {
      const target = get().nodes.find((n) => n.id === id);
      if (!target) return;
      checkpoint();
      set({
        nodes: get().nodes.map((n) =>
          n.id === id
            ? { ...n, data: { ...n.data, config: { ...n.data.config, ...patch } } }
            : n,
        ),
      });
    },

    deleteSelection: () => {
      const { nodes, edges } = get();
      const doomedNodes = new Set(
        nodes.filter((n) => n.selected && n.deletable !== false).map((n) => n.id),
      );
      const doomedEdges = new Set(
        edges
          .filter(
            (e) => e.selected || doomedNodes.has(e.source) || doomedNodes.has(e.target),
          )
          .map((e) => e.id),
      );
      if (doomedNodes.size === 0 && doomedEdges.size === 0) return;
      checkpoint();
      set({
        nodes: nodes.filter((n) => !doomedNodes.has(n.id)),
        edges: edges.filter((e) => !doomedEdges.has(e.id)),
      });
    },

    copySelection: () => {
      const { nodes, edges } = get();
      const selected = nodes.filter((n) => n.selected);
      if (selected.length === 0) return;
      const ids = new Set(selected.map((n) => n.id));
      set({
        clipboard: {
          nodes: structuredClone(selected),
          edges: structuredClone(
            edges.filter((e) => ids.has(e.source) && ids.has(e.target)),
          ),
        },
      });
    },

    pasteClipboard: () => {
      const { clipboard } = get();
      if (!clipboard || clipboard.nodes.length === 0) return;
      checkpoint();
      insertCopies(get, set, clipboard.nodes, clipboard.edges);
    },

    duplicateSelection: () => {
      const { nodes, edges } = get();
      const selected = nodes.filter((n) => n.selected && n.deletable !== false);
      if (selected.length === 0) return;
      const ids = new Set(selected.map((n) => n.id));
      checkpoint();
      insertCopies(
        get,
        set,
        structuredClone(selected),
        structuredClone(edges.filter((e) => ids.has(e.source) && ids.has(e.target))),
      );
    },

    selectAll: () => {
      set({
        nodes: get().nodes.map((n) => ({ ...n, selected: true })),
        edges: get().edges.map((e) => ({ ...e, selected: true })),
      });
    },

    undo: () => {
      const { past, future, nodes, edges, counters } = get();
      const snapshot = past[past.length - 1];
      if (!snapshot) return;
      set({
        past: past.slice(0, -1),
        future: [
          { nodes: structuredClone(nodes), edges: structuredClone(edges), counters: { ...counters } },
          ...future,
        ].slice(0, HISTORY_LIMIT),
        nodes: snapshot.nodes,
        edges: snapshot.edges,
        counters: snapshot.counters,
      });
    },

    redo: () => {
      const { past, future, nodes, edges, counters } = get();
      const snapshot = future[0];
      if (!snapshot) return;
      set({
        future: future.slice(1),
        past: [
          ...past,
          { nodes: structuredClone(nodes), edges: structuredClone(edges), counters: { ...counters } },
        ].slice(-HISTORY_LIMIT),
        nodes: snapshot.nodes,
        edges: snapshot.edges,
        counters: snapshot.counters,
      });
    },

    setNodes: (nodes) => set({ nodes }),
    setEdges: (edges) => set({ edges }),
  };
});
