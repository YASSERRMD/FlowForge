"use client";

import * as React from "react";
import { toast } from "sonner";

import { countersFromNodes, fromWorkflowGraph, toWorkflowGraph } from "@/lib/editor/convert";
import { exportGraphDocument, parseGraphDocument } from "@/lib/editor/import";
import { useEditorStore } from "@/lib/editor/store";
import { Button } from "@/components/ui/button";

/** Import/export buttons with size-bounded parsing and clear error surfacing. */
export function ImportExportControls() {
  const fileRef = React.useRef<HTMLInputElement>(null);

  const onExport = () => {
    const { nodes, edges } = useEditorStore.getState();
    const doc = exportGraphDocument(toWorkflowGraph(nodes, edges));
    const blob = new Blob([doc], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = "workflow-graph.json";
    a.click();
    URL.revokeObjectURL(url);
  };

  const onFile = async (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    event.target.value = "";
    if (!file) return;

    const text = await file.text();
    const result = parseGraphDocument(text);
    if (!result.ok) {
      toast.error("Import failed", {
        description:
          result.error +
          (result.issues
            ? "\n" + result.issues.slice(0, 5).map((i) => `• ${i.message}`).join("\n") +
              (result.issues.length > 5 ? `\n• …and ${result.issues.length - 5} more` : "")
            : ""),
      });
      return;
    }

    const { nodes, edges } = fromWorkflowGraph(result.graph);
    useEditorStore.setState({
      nodes,
      edges,
      counters: countersFromNodes(nodes) as never,
      past: [],
      future: [],
    });
    toast.success("Graph imported", {
      description:
        result.warnings.length > 0
          ? `${result.warnings.length} warning${result.warnings.length === 1 ? "" : "s"} — see the validation panel.`
          : undefined,
    });
  };

  return (
    <div className="flex items-center gap-2">
      <Button type="button" variant="outline" size="sm" onClick={onExport}>
        Export
      </Button>
      <Button
        type="button"
        variant="outline"
        size="sm"
        onClick={() => fileRef.current?.click()}
      >
        Import
      </Button>
      <input
        ref={fileRef}
        type="file"
        accept="application/json,.json"
        className="hidden"
        aria-label="Import graph JSON"
        onChange={(e) => void onFile(e)}
      />
    </div>
  );
}
