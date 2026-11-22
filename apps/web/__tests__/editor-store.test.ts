import { beforeEach, describe, expect, it } from "vitest";

import { useEditorStore } from "@/lib/editor/store";

const initial = useEditorStore.getState();

beforeEach(() => {
  useEditorStore.setState(initial, true);
});

describe("editor store", () => {
  it("starts with a start and end node", () => {
    const { nodes } = useEditorStore.getState();
    expect(nodes.map((n) => n.id)).toEqual(["start", "end"]);
  });

  it("assigns deterministic sequential ids per kind", () => {
    const s = useEditorStore.getState();
    const a1 = s.addNode("action", { x: 0, y: 0 });
    const a2 = s.addNode("action", { x: 10, y: 0 });
    const c1 = s.addNode("condition", { x: 20, y: 0 });
    expect(a1.id).toBe("action-1");
    expect(a2.id).toBe("action-2");
    expect(c1.id).toBe("condition-1");
    expect(a1.data.label).toBe("Action 1");
  });

  it("creates edges with deterministic ids on connect", () => {
    const s = useEditorStore.getState();
    s.addNode("action", { x: 0, y: 0 });
    s.onConnect({ source: "start", target: "action-1", sourceHandle: null, targetHandle: null });
    const { edges } = useEditorStore.getState();
    expect(edges).toHaveLength(1);
    expect(edges[0].id).toBe("e-start-out-action-1-in");
  });
});

describe("condition branches", () => {
  it("labels edges from condition handles with their branch", () => {
    const s = useEditorStore.getState();
    const cond = s.addNode("condition", { x: 0, y: 0 });
    const act = s.addNode("action", { x: 0, y: 100 });
    s.onConnect({
      source: cond.id,
      sourceHandle: "true",
      target: act.id,
      targetHandle: "in",
    });
    const { edges } = useEditorStore.getState();
    expect(edges[0].label).toBe("true");
    expect(edges[0].data).toEqual({ branch: "true" });
  });

  it("does not label edges from non-condition nodes", () => {
    const s = useEditorStore.getState();
    const act = s.addNode("action", { x: 0, y: 0 });
    s.onConnect({ source: "start", sourceHandle: "out", target: act.id, targetHandle: "in" });
    expect(useEditorStore.getState().edges[0].label).toBeUndefined();
  });
});
