"use client";

import * as React from "react";
import { useReactFlow } from "@xyflow/react";

import { useEditorStore } from "@/lib/editor/store";
import { toWorkflowGraph } from "@/lib/editor/convert";
import { validateGraph, type GraphIssue } from "@/lib/editor/validate";
import { cn } from "@/lib/utils";

function IssueRow({
  issue,
  onFocus,
}: {
  issue: GraphIssue;
  onFocus: (issue: GraphIssue) => void;
}) {
  const target = issue.nodeId ?? issue.edgeId;
  return (
    <li>
      <button
        type="button"
        disabled={!target}
        onClick={() => onFocus(issue)}
        title={target ? "Click to focus" : undefined}
        className={cn(
          "flex w-full items-start gap-2 rounded-md px-2 py-1.5 text-left text-xs",
          target ? "hover:bg-accent" : "cursor-default",
        )}
      >
        <span
          aria-hidden
          className={cn(
            "mt-1.5 inline-block h-2 w-2 shrink-0 rounded-full",
            issue.severity === "error" ? "bg-error" : "bg-warning",
          )}
        />
        <span className="flex-1">
          <span className="font-medium text-foreground">{issue.code}</span>
          <span className="text-muted-foreground"> — {issue.message}</span>
        </span>
      </button>
    </li>
  );
}

/**
 * Live validation panel: re-validates the graph on every edit and focuses
 * the offending node or edge on click.
 */
export function ValidationInspector() {
  const nodes = useEditorStore((s) => s.nodes);
  const edges = useEditorStore((s) => s.edges);
  const setNodes = useEditorStore((s) => s.setNodes);
  const setEdges = useEditorStore((s) => s.setEdges);
  const { setCenter, fitView } = useReactFlow();
  const [open, setOpen] = React.useState(true);

  const issues = React.useMemo(
    () => validateGraph(toWorkflowGraph(nodes, edges)),
    [nodes, edges],
  );
  const errors = issues.filter((i) => i.severity === "error").length;
  const warnings = issues.length - errors;

  const focus = React.useCallback(
    (issue: GraphIssue) => {
      if (issue.nodeId) {
        const state = useEditorStore.getState();
        setNodes(state.nodes.map((n) => ({ ...n, selected: n.id === issue.nodeId })));
        setEdges(state.edges.map((e) => ({ ...e, selected: false })));
        const node = state.nodes.find((n) => n.id === issue.nodeId);
        if (node) {
          void setCenter(node.position.x + 100, node.position.y + 40, {
            zoom: 1.2,
            duration: 300,
          });
        }
      } else if (issue.edgeId) {
        const state = useEditorStore.getState();
        setEdges(state.edges.map((e) => ({ ...e, selected: e.id === issue.edgeId })));
        setNodes(state.nodes.map((n) => ({ ...n, selected: false })));
        void fitView({ duration: 300, padding: 0.3 });
      }
    },
    [setNodes, setEdges, setCenter, fitView],
  );

  return (
    <section
      aria-label="Graph validation"
      className="rounded-lg border border-border bg-card"
    >
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
        className="flex w-full items-center justify-between px-3 py-2 text-sm font-medium text-foreground"
      >
        <span>
          Validation{" "}
          {issues.length === 0 ? (
            <span className="text-success">— no issues</span>
          ) : (
            <span className="text-muted-foreground">
              — {errors} error{errors === 1 ? "" : "s"}, {warnings} warning
              {warnings === 1 ? "" : "s"}
            </span>
          )}
        </span>
        <span aria-hidden className="text-muted-foreground">
          {open ? "▾" : "▸"}
        </span>
      </button>
      {open && issues.length > 0 ? (
        <ul className="max-h-48 overflow-y-auto border-t border-border p-1">
          {issues.map((issue, i) => (
            <IssueRow
              key={`${issue.code}-${issue.nodeId ?? issue.edgeId ?? i}`}
              issue={issue}
              onFocus={focus}
            />
          ))}
        </ul>
      ) : null}
    </section>
  );
}
