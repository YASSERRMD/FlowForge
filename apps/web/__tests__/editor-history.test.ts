import { beforeEach, describe, expect, it } from "vitest";

import { useEditorStore } from "@/lib/editor/store";

const initial = useEditorStore.getState();

beforeEach(() => {
  useEditorStore.setState(initial, true);
});

const s = () => useEditorStore.getState();

describe("undo/redo history", () => {
  it("undoes and redoes node creation", () => {
    s().addNode("action", { x: 0, y: 0 });
    expect(s().nodes).toHaveLength(3);

    s().undo();
    expect(s().nodes).toHaveLength(2);

    s().redo();
    expect(s().nodes).toHaveLength(3);
    expect(s().nodes[2].id).toBe("action-1");
  });

  it("restores counters on undo so ids stay deterministic", () => {
    s().addNode("action", { x: 0, y: 0 });
    s().undo();
    const n = s().addNode("action", { x: 5, y: 5 });
    expect(n.id).toBe("action-1"); // not action-2
  });

  it("undoes edge creation and reconnect", () => {
    const a = s().addNode("action", { x: 0, y: 0 });
    s().onConnect({ source: "start", target: a.id, sourceHandle: "out", targetHandle: "in" });
    expect(s().edges).toHaveLength(1);

    s().undo();
    expect(s().edges).toHaveLength(0);
    s().redo();
    expect(s().edges).toHaveLength(1);

    const edge = s().edges[0];
    s().onReconnect(edge, { source: "start", target: "end", sourceHandle: "out", targetHandle: "in" });
    expect(s().edges[0].target).toBe("end");
    s().undo();
    expect(s().edges[0].target).toBe(a.id);
  });

  it("undoes label edits", () => {
    const a = s().addNode("action", { x: 0, y: 0 });
    s().updateNodeLabel(a.id, "Charge card");
    expect(s().nodes.find((n) => n.id === a.id)?.data.label).toBe("Charge card");
    s().undo();
    expect(s().nodes.find((n) => n.id === a.id)?.data.label).toBe("Action 1");
  });

  it("undoes deletion with attached edges", () => {
    const a = s().addNode("action", { x: 0, y: 0 });
    s().onConnect({ source: "start", target: a.id, sourceHandle: "out", targetHandle: "in" });
    s().setNodes(s().nodes.map((n) => (n.id === a.id ? { ...n, selected: true } : n)));
    s().deleteSelection();
    expect(s().nodes).toHaveLength(2);
    expect(s().edges).toHaveLength(0);

    s().undo();
    expect(s().nodes).toHaveLength(3);
    expect(s().edges).toHaveLength(1);
  });

  it("never deletes start/end via deleteSelection", () => {
    s().setNodes(s().nodes.map((n) => ({ ...n, selected: true })));
    s().deleteSelection();
    expect(s().nodes.map((n) => n.id)).toEqual(["start", "end"]);
  });

  it("clears redo stack on new edits", () => {
    s().addNode("action", { x: 0, y: 0 });
    s().undo();
    s().addNode("delay", { x: 0, y: 0 });
    expect(s().future).toHaveLength(0);
    s().redo(); // no-op
    expect(s().nodes).toHaveLength(3);
  });

  it("bounds history to the limit", () => {
    for (let i = 0; i < 60; i++) {
      s().updateNodeLabel("start", `S${i}`);
    }
    expect(s().past.length).toBeLessThanOrEqual(50);
  });

  it("undo on empty history is a no-op", () => {
    s().undo();
    expect(s().nodes).toHaveLength(2);
  });
});
