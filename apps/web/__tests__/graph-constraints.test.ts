import { describe, expect, it } from "vitest";

import { isValidConnection, wouldCreateCycle } from "@/lib/editor/graph";

const nodes = [
  { id: "start", type: "start" },
  { id: "end", type: "end" },
  { id: "a", type: "action" },
  { id: "b", type: "action" },
  { id: "c", type: "condition" },
];

const edges = [
  { id: "e1", source: "start", target: "a", sourceHandle: null, targetHandle: null },
  { id: "e2", source: "a", target: "b", sourceHandle: null, targetHandle: null },
];

describe("wouldCreateCycle", () => {
  it("detects a direct back edge", () => {
    expect(wouldCreateCycle(edges, "b", "a")).toBe(true);
  });
  it("detects an indirect cycle", () => {
    expect(wouldCreateCycle(edges, "b", "start")).toBe(true);
  });
  it("allows forward edges", () => {
    expect(wouldCreateCycle(edges, "b", "end")).toBe(false);
  });
});

describe("isValidConnection", () => {
  it("rejects self loops", () => {
    expect(isValidConnection({ source: "a", target: "a", sourceHandle: null, targetHandle: null }, nodes, edges)).toBe(false);
  });
  it("rejects edges into start", () => {
    expect(isValidConnection({ source: "a", target: "start", sourceHandle: null, targetHandle: null }, nodes, edges)).toBe(false);
  });
  it("rejects edges out of end", () => {
    expect(isValidConnection({ source: "end", target: "a", sourceHandle: null, targetHandle: null }, nodes, edges)).toBe(false);
  });
  it("rejects duplicate edges", () => {
    expect(isValidConnection({ source: "start", target: "a", sourceHandle: null, targetHandle: null }, nodes, edges)).toBe(false);
  });
  it("rejects cycles", () => {
    expect(isValidConnection({ source: "b", target: "a", sourceHandle: null, targetHandle: null }, nodes, edges)).toBe(false);
  });
  it("allows valid new edges", () => {
    expect(isValidConnection({ source: "b", target: "end", sourceHandle: null, targetHandle: null }, nodes, edges)).toBe(true);
  });
  it("allows reconnecting an edge to its own endpoints (no false duplicate/cycle)", () => {
    expect(
      isValidConnection(
        { source: "start", target: "a", sourceHandle: null, targetHandle: null },
        nodes,
        edges,
        "e1",
      ),
    ).toBe(true);
  });
});
