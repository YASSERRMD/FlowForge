import { describe, expect, it } from "vitest";

import type { WorkflowGraph } from "@/lib/contracts/graph";
import { countersFromNodes, fromWorkflowGraph, toWorkflowGraph } from "./convert";

const GRAPH: WorkflowGraph = {
  schemaVersion: "1",
  nodes: [
    { id: "start-1", kind: "start", label: "Start", position: { x: 0, y: 0 } },
    {
      id: "action-3",
      kind: "action",
      label: "Fetch",
      position: { x: 40, y: 120 },
      config: { actionType: "http.request", params: { url: "https://x" }, futureKey: true },
    },
    { id: "end-1", kind: "end", label: "End", position: { x: 0, y: 300 } },
  ],
  edges: [
    { id: "e-1", source: "start-1", sourceHandle: null, target: "action-3", targetHandle: null },
    { id: "e-2", source: "action-3", sourceHandle: null, target: "end-1", targetHandle: null, label: "done" },
  ],
};

describe("graph conversion round-trip", () => {
  it("graph → editor → graph preserves every field", () => {
    const { nodes, edges } = fromWorkflowGraph(GRAPH);
    const back = toWorkflowGraph(nodes, edges);
    expect(back.schemaVersion).toBe("1");
    expect(back.nodes).toHaveLength(3);
    const action = back.nodes.find((n) => n.id === "action-3");
    expect(action).toMatchObject({
      kind: "action",
      label: "Fetch",
      position: { x: 40, y: 120 },
    });
    // Unknown config keys survive untouched.
    expect(action?.config?.futureKey).toBe(true);
    expect(back.edges[1]).toMatchObject({ id: "e-2", label: "done" });
  });

  it("editor → graph → editor → graph is stable", () => {
    const first = fromWorkflowGraph(GRAPH);
    const once = toWorkflowGraph(first.nodes, first.edges);
    const second = fromWorkflowGraph(once);
    const twice = toWorkflowGraph(second.nodes, second.edges);
    expect(twice).toEqual(once);
  });

  it("serializes deterministically (byte-stable)", () => {
    const { nodes, edges } = fromWorkflowGraph(GRAPH);
    const graph = toWorkflowGraph(nodes, edges);
    expect(JSON.stringify(graph)).toBe(JSON.stringify(toWorkflowGraph(nodes, edges)));
  });

  it("marks start/end nodes non-deletable", () => {
    const { nodes } = fromWorkflowGraph(GRAPH);
    expect(nodes.find((n) => n.id === "start-1")?.deletable).toBe(false);
    expect(nodes.find((n) => n.id === "end-1")?.deletable).toBe(false);
    expect(nodes.find((n) => n.id === "action-3")?.deletable).toBeUndefined();
  });

  it("recomputes counters so new IDs never collide", () => {
    const { nodes } = fromWorkflowGraph(GRAPH);
    const counters = countersFromNodes(nodes);
    expect(counters).toMatchObject({ start: 1, end: 1, action: 3 });
  });

  it("falls back to a grid layout when positions are missing", () => {
    const noPos: WorkflowGraph = {
      schemaVersion: "1",
      nodes: [
        { id: "start-1", kind: "start", label: "Start" },
        { id: "end-1", kind: "end", label: "End" },
      ],
      edges: [],
    };
    const { nodes } = fromWorkflowGraph(noPos);
    expect(nodes[0].position).toEqual({ x: 0, y: 0 });
    expect(nodes[1].position).toEqual({ x: 220, y: 0 });
  });
});
