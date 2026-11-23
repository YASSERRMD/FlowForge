import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ReactFlowProvider } from "@xyflow/react";
import { beforeEach, describe, expect, it } from "vitest";

import { ValidationInspector } from "@/components/editor/validation-inspector";
import { useEditorStore } from "@/lib/editor/store";

function renderInspector() {
  return render(
    <ReactFlowProvider>
      <ValidationInspector />
    </ReactFlowProvider>,
  );
}

describe("ValidationInspector", () => {
  beforeEach(() => {
    // Minimal valid graph: start → end.
    useEditorStore.setState({
      nodes: [
        { id: "start", type: "start", position: { x: 0, y: 0 }, data: { label: "Start", kind: "start" }, deletable: false },
        { id: "end", type: "end", position: { x: 0, y: 300 }, data: { label: "End", kind: "end" }, deletable: false },
      ],
      edges: [
        { id: "e-1", source: "start", target: "end" },
      ],
      past: [],
      future: [],
    });
  });

  it("reports no issues for a valid graph", () => {
    renderInspector();
    expect(screen.getByText(/no issues/i)).toBeInTheDocument();
  });

  it("lists issues for an invalid graph and focuses the node on click", async () => {
    useEditorStore.setState({
      nodes: [
        { id: "start", type: "start", position: { x: 0, y: 0 }, data: { label: "Start", kind: "start" }, deletable: false },
        { id: "end", type: "end", position: { x: 0, y: 300 }, data: { label: "End", kind: "end" }, deletable: false },
        { id: "action-1", type: "action", position: { x: 50, y: 50 }, data: { label: "Lonely", kind: "action", config: { actionType: "http.request" } } },
      ],
      edges: [{ id: "e-1", source: "start", target: "end" }],
    });
    renderInspector();

    expect(screen.getByText(/unreachable_node/)).toBeInTheDocument();

    await userEvent.click(screen.getAllByRole("button", { name: /unreachable_node/ })[0]);
    const actionNode = useEditorStore.getState().nodes.find((n) => n.id === "action-1");
    expect(actionNode?.selected).toBe(true);
  });

  it("collapses and expands the issue list", async () => {
    useEditorStore.setState({
      nodes: [
        { id: "start", type: "start", position: { x: 0, y: 0 }, data: { label: "Start", kind: "start" }, deletable: false },
      ],
      edges: [],
    });
    renderInspector();
    const toggle = screen.getByRole("button", { name: /Validation/ });
    expect(screen.getByText(/missing_end/)).toBeInTheDocument();
    await userEvent.click(toggle);
    expect(screen.queryByText(/missing_end/)).not.toBeInTheDocument();
  });
});
