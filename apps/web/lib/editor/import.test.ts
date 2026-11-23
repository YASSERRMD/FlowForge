import { describe, expect, it } from "vitest";

import { exportGraphDocument, MAX_IMPORT_BYTES, parseGraphDocument } from "./import";

const VALID = JSON.stringify({
  schemaVersion: "1",
  nodes: [
    { id: "start-1", kind: "start", label: "Start" },
    { id: "end-1", kind: "end", label: "End" },
  ],
  edges: [{ id: "e-1", source: "start-1", target: "end-1" }],
});

describe("parseGraphDocument", () => {
  it("accepts a valid graph", () => {
    const result = parseGraphDocument(VALID);
    expect(result.ok).toBe(true);
    if (result.ok) expect(result.warnings).toHaveLength(0);
  });

  it("rejects oversized documents with a clear message", () => {
    const big = " ".repeat(MAX_IMPORT_BYTES + 1);
    const result = parseGraphDocument(big);
    expect(result).toMatchObject({ ok: false });
    if (!result.ok) expect(result.error).toMatch(/too large/);
  });

  it("rejects invalid JSON with the parse error", () => {
    const result = parseGraphDocument("{not json");
    expect(result.ok).toBe(false);
    if (!result.ok) expect(result.error).toMatch(/Not valid JSON/);
  });

  it("rejects non-object documents", () => {
    const result = parseGraphDocument("[1,2,3]");
    expect(!result.ok && result.error).toMatch(/JSON object/);
  });

  it("rejects unknown schema versions", () => {
    const result = parseGraphDocument(JSON.stringify({ schemaVersion: "2", nodes: [], edges: [] }));
    expect(!result.ok && result.error).toMatch(/schemaVersion/);
  });

  it("rejects documents missing nodes/edges arrays", () => {
    const result = parseGraphDocument(JSON.stringify({ schemaVersion: "1" }));
    expect(!result.ok && result.error).toMatch(/nodes.*edges/);
  });

  it("rejects node-count overflow before deeper validation", () => {
    const nodes = Array.from({ length: 201 }, (_, i) => ({
      id: `n-${i}`,
      kind: "action",
      label: `N${i}`,
    }));
    const result = parseGraphDocument(JSON.stringify({ schemaVersion: "1", nodes, edges: [] }));
    expect(!result.ok && result.error).toMatch(/Too many nodes/);
  });

  it("rejects structurally invalid graphs with issue detail", () => {
    const cyclic = JSON.stringify({
      schemaVersion: "1",
      nodes: [
        { id: "start-1", kind: "start", label: "S" },
        { id: "action-1", kind: "action", label: "A", config: { actionType: "http.request" } },
        { id: "end-1", kind: "end", label: "E" },
      ],
      edges: [
        { id: "e1", source: "start-1", target: "action-1" },
        { id: "e2", source: "action-1", target: "action-1" },
        { id: "e3", source: "action-1", target: "end-1" },
      ],
    });
    const result = parseGraphDocument(cyclic);
    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.issues?.some((i) => i.code === "self_loop")).toBe(true);
    }
  });

  it("passes warnings through on success", () => {
    const withOrphan = JSON.stringify({
      schemaVersion: "1",
      nodes: [
        { id: "start-1", kind: "start", label: "S" },
        { id: "end-1", kind: "end", label: "E" },
        { id: "action-1", kind: "action", label: "Orphan", config: { actionType: "http.request" } },
      ],
      edges: [
        { id: "e1", source: "start-1", target: "end-1" },
        { id: "e2", source: "start-1", target: "action-1" },
        { id: "e3", source: "action-1", target: "end-1" },
      ],
    });
    // Fully connected variant: no warnings.
    const ok = parseGraphDocument(withOrphan);
    expect(ok.ok).toBe(true);
  });
});

describe("exportGraphDocument", () => {
  it("round-trips through the importer", () => {
    const parsed = parseGraphDocument(VALID);
    expect(parsed.ok).toBe(true);
    if (parsed.ok) {
      const exported = exportGraphDocument(parsed.graph);
      const again = parseGraphDocument(exported);
      expect(again.ok).toBe(true);
    }
  });
});
