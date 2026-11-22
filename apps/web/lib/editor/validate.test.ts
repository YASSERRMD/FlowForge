import { describe, expect, it } from "vitest";

import type { WorkflowGraph } from "@/lib/contracts/graph";
import { validateGraph } from "./validate";

function baseGraph(): WorkflowGraph {
  return {
    schemaVersion: "1",
    nodes: [
      { id: "start-1", kind: "start", label: "Start" },
      { id: "action-1", kind: "action", label: "Fetch", config: { actionType: "http.request" } },
      { id: "end-1", kind: "end", label: "End" },
    ],
    edges: [
      { id: "e-1", source: "start-1", target: "action-1" },
      { id: "e-2", source: "action-1", target: "end-1" },
    ],
  };
}

function codes(graph: WorkflowGraph) {
  return validateGraph(graph).map((i) => i.code);
}

describe("validateGraph", () => {
  it("accepts a minimal valid graph", () => {
    expect(validateGraph(baseGraph())).toEqual([]);
  });

  it("rejects unknown schema versions", () => {
    const g = { ...baseGraph(), schemaVersion: "2" } as unknown as WorkflowGraph;
    expect(codes(g)).toContain("schema_version");
  });

  it("requires exactly one start and at least one end", () => {
    const noStart = baseGraph();
    noStart.nodes = noStart.nodes.filter((n) => n.kind !== "start");
    noStart.edges = noStart.edges.filter((e) => e.source !== "start-1");
    expect(codes(noStart)).toContain("missing_start");

    const twoStarts = baseGraph();
    twoStarts.nodes.push({ id: "start-2", kind: "start", label: "Start 2" });
    expect(codes(twoStarts)).toContain("multiple_start");

    const noEnd = baseGraph();
    noEnd.nodes = noEnd.nodes.filter((n) => n.kind !== "end");
    noEnd.edges = noEnd.edges.filter((e) => e.target !== "end-1");
    expect(codes(noEnd)).toContain("missing_end");
  });

  it("flags unreachable nodes", () => {
    const g = baseGraph();
    g.nodes.push({ id: "action-2", kind: "action", label: "Detached", config: { actionType: "http.request" } });
    const issueCodes = codes(g);
    expect(issueCodes).toContain("unreachable_node");
    expect(issueCodes).toContain("orphan_node");
  });

  it("flags cycles", () => {
    const g = baseGraph();
    g.edges.push({ id: "e-3", source: "action-1", target: "action-1" });
    expect(codes(g)).toContain("self_loop");
  });

  it("detects real cycles across nodes", () => {
    const g = baseGraph();
    g.nodes.push({ id: "action-2", kind: "action", label: "B", config: { actionType: "http.request" } });
    g.edges.push({ id: "e-3", source: "action-1", target: "action-2" });
    g.edges.push({ id: "e-4", source: "action-2", target: "action-1" });
    expect(codes(g)).toContain("cycle");
  });

  it("flags duplicate node and edge IDs", () => {
    const g = baseGraph();
    g.nodes.push({ id: "action-1", kind: "action", label: "Dup" });
    g.edges.push({ id: "e-1", source: "action-1", target: "end-1" });
    const issueCodes = codes(g);
    expect(issueCodes).toContain("duplicate_node_id");
    expect(issueCodes).toContain("duplicate_edge_id");
    expect(issueCodes).toContain("duplicate_edge");
  });

  it("flags dangling edges", () => {
    const g = baseGraph();
    g.edges.push({ id: "e-9", source: "ghost", target: "end-1" });
    expect(codes(g)).toContain("dangling_edge");
  });

  it("enforces port rules per kind", () => {
    const g = baseGraph();
    g.nodes.push({
      id: "condition-1",
      kind: "condition",
      label: "C",
      config: { expression: "$trigger.a > 1" },
    });
    g.edges.push({ id: "e-3", source: "start-1", target: "condition-1" });
    g.edges.push({ id: "e-4", source: "condition-1", sourceHandle: "maybe", target: "end-1" });
    expect(codes(g)).toContain("invalid_source_handle");
    g.edges[3].sourceHandle = "true";
    expect(codes(g)).not.toContain("invalid_source_handle");
  });

  it("forbids edges out of end and into start", () => {
    const g = baseGraph();
    g.nodes.push({ id: "end-2", kind: "end", label: "End 2" });
    g.edges.push({ id: "e-3", source: "end-1", target: "end-2" });
    g.edges.push({ id: "e-4", source: "action-1", target: "start-1" });
    const issueCodes = codes(g);
    expect(issueCodes).toContain("invalid_source_handle");
    expect(issueCodes).toContain("invalid_target_kind");
  });

  it("enforces size limits", () => {
    const g = baseGraph();
    for (let i = 0; i < 201; i++) {
      g.nodes.push({ id: `action-x${i}`, kind: "action", label: `A${i}` });
    }
    expect(codes(g)).toContain("size_limit");
  });

  it("surfaces config issues as node-scoped errors", () => {
    const g = baseGraph();
    g.nodes[1].config = {};
    const issue = validateGraph(g).find((i) => i.code === "config");
    expect(issue?.nodeId).toBe("action-1");
  });
});
