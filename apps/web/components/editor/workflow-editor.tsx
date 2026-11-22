"use client";

import * as React from "react";
import {
  ReactFlow,
  ReactFlowProvider,
  Background,
  BackgroundVariant,
  useReactFlow,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";

import { useEditorStore } from "@/lib/editor/store";
import type { NodeKind } from "@/lib/editor/types";
import { nodeTypes } from "./nodes";
import { NodePalette, DND_MIME } from "./palette";

function Canvas() {
  const nodes = useEditorStore((s) => s.nodes);
  const edges = useEditorStore((s) => s.edges);
  const onNodesChange = useEditorStore((s) => s.onNodesChange);
  const onEdgesChange = useEditorStore((s) => s.onEdgesChange);
  const onConnect = useEditorStore((s) => s.onConnect);
  const addNode = useEditorStore((s) => s.addNode);

  const wrapper = React.useRef<HTMLDivElement>(null);
  const { screenToFlowPosition } = useReactFlow();

  const addAt = React.useCallback(
    (kind: NodeKind, clientX?: number, clientY?: number) => {
      const rect = wrapper.current?.getBoundingClientRect();
      const x = clientX ?? (rect ? rect.left + rect.width / 2 : 0);
      const y = clientY ?? (rect ? rect.top + rect.height / 2 : 0);
      addNode(kind, screenToFlowPosition({ x, y }));
    },
    [addNode, screenToFlowPosition],
  );

  return (
    <div className="flex flex-col gap-3">
      <NodePalette onAdd={(kind) => addAt(kind)} />
      <div
        ref={wrapper}
        className="h-[calc(100vh-12rem)] w-full rounded-lg border border-border"
        aria-label="Workflow graph canvas"
        onDragOver={(event) => {
          if (event.dataTransfer.types.includes(DND_MIME)) {
            event.preventDefault();
            event.dataTransfer.dropEffect = "move";
          }
        }}
        onDrop={(event) => {
          const kind = event.dataTransfer.getData(DND_MIME) as NodeKind;
          if (!kind) return;
          event.preventDefault();
          addAt(kind, event.clientX, event.clientY);
        }}
      >
        <ReactFlow
          nodes={nodes}
          edges={edges}
          nodeTypes={nodeTypes}
          onNodesChange={onNodesChange}
          onEdgesChange={onEdgesChange}
          onConnect={onConnect}
          fitView
          proOptions={{ hideAttribution: false }}
        >
          <Background variant={BackgroundVariant.Dots} gap={16} size={1} />
        </ReactFlow>
      </div>
    </div>
  );
}

export function WorkflowEditor() {
  return (
    <ReactFlowProvider>
      <Canvas />
    </ReactFlowProvider>
  );
}
