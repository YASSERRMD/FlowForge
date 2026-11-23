import { describe, expect, it } from "vitest";

import type { WorkflowGraph } from "@/lib/contracts/graph";
import { diffGraphs, summarizeDiff } from "./diff";

const V1: WorkflowGraph = {
  schemaVersion: "1",
  nodes: [
    { id: "start-1", kind: "start", label: "Start", position: { x: 0, y: 0 } },
    { id: "action-1", kind: "action", label: "Fetch", position: { x: 0, y: 100 }, config: { actionType: "http.request" } },
    { id: "end-1", kind: "end", label: "End", position: { x: 0, y: 200 } },
  ],
  edges: [
    { id: "e-1", source: "start-1", target: "action-1" },
    { id: "e-2", source: "action-1", target: "end-1" },
  ],
};

describe("diffGraphs", () => {
  it("reports identical graphs", () => {
    const diff = diffGraphs(V1, V1);
    expect(diff.identical).toBe(true);
    expect(summarizeDiff(diff)).toBe("No differences");
  });

  it("detects added and removed nodes", () => {
    const v2: WorkflowGraph = {
      ...V1,
      nodes: [
        ...V1.nodes.filter((n) => n.id !== "action-1"),
        { id: "delay-1", kind: "delay", label: "Wait", config: { durationMs: 1000 } },
      ],
      edges: [
        { id: "e-3", source: "start-1", target: "delay-1" },
        { id: "e-4", source: "delay-1", target: "end-1" },
      ],
    };
    const diff = diffGraphs(V1, v2);
    expect(diff.nodesAdded.map((n) => n.id)).toEqual(["delay-1"]);
    expect(diff.nodesRemoved.map((n) => n.id)).toEqual(["action-1"]);
    expect(diff.edgesAdded).toHaveLength(2);
    expect(diff.edgesRemoved).toHaveLength(2);
    expect(summarizeDiff(diff)).toBe("1 node added, 1 node removed, 2 edges added, 2 edges removed");
  });

  it("detects label, config and position changes", () => {
    const v2: WorkflowGraph = {
      ...V1,
      nodes: V1.nodes.map((n) =>
        n.id === "action-1"
          ? { ...n, label: "Fetch customer", position: { x: 5, y: 100 }, config: { actionType: "email.send" } }
          : n,
      ),
    };
    const diff = diffGraphs(V1, v2);
    expect(diff.nodesChanged).toHaveLength(1);
    expect(diff.nodesChanged[0].changes).toContain('label: "Fetch" → "Fetch customer"');
    expect(diff.nodesChanged[0].changes).toContain("configuration changed");
    expect(diff.nodesChanged[0].changes).toContain("moved on canvas");
  });

  it("ignores config key ordering", () => {
    const v2: WorkflowGraph = {
      ...V1,
      nodes: V1.nodes.map((n) =>
        n.id === "action-1" ? { ...n, config: { actionType: "http.request" } } : n,
      ),
    };
    expect(diffGraphs(V1, v2).identical).toBe(true);
  });
});
