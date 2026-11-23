"use client";

import * as React from "react";
import {
  ReactFlow,
  ReactFlowProvider,
  Background,
  BackgroundVariant,
  Controls,
  MiniMap,
  SelectionMode,
  useReactFlow,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";

import { useEditorStore } from "@/lib/editor/store";
import type { NodeKind } from "@/lib/editor/types";
import { nodeTypes } from "./nodes";
import { NodeConfigPanel } from "./config-panel";
import { ImportExportControls } from "./import-export";
import { ValidationInspector } from "./validation-inspector";
import { NodePalette, DND_MIME } from "./palette";
import { useEditorKeyboard } from "./use-editor-keyboard";

function Canvas() {
  useEditorKeyboard();
  const nodes = useEditorStore((s) => s.nodes);
  const edges = useEditorStore((s) => s.edges);
  const onNodesChange = useEditorStore((s) => s.onNodesChange);
  const onEdgesChange = useEditorStore((s) => s.onEdgesChange);
  const onConnect = useEditorStore((s) => s.onConnect);
  const onReconnect = useEditorStore((s) => s.onReconnect);
  const canConnect = useEditorStore((s) => s.canConnect);
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
      <div className="flex flex-wrap items-center justify-between gap-3">
        <NodePalette onAdd={(kind) => addAt(kind)} />
        <ImportExportControls />
      </div>
      <div className="flex items-start gap-3">
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
          onReconnect={onReconnect}
          isValidConnection={(conn) => canConnect(conn)}
          edgesReconnectable
          reconnectRadius={12}
          fitView
          minZoom={0.2}
          maxZoom={2}
          snapToGrid
          snapGrid={[16, 16]}
          panOnDrag={[1, 2]}
          selectionOnDrag
          selectionMode={SelectionMode.Partial}
          multiSelectionKeyCode={["Shift", "Meta", "Control"]}
          deleteKeyCode={null}
          proOptions={{ hideAttribution: false }}
        >
          <Background variant={BackgroundVariant.Dots} gap={16} size={1} />
          <Controls position="bottom-left" aria-label="Canvas controls" />
          <MiniMap
            position="bottom-right"
            pannable
            zoomable
            aria-label="Canvas minimap"
            className="!bg-muted"
          />
        </ReactFlow>
        </div>
        <div className="flex w-72 shrink-0 flex-col gap-3 empty:hidden">
          <NodeConfigPanel />
        </div>
      </div>
      <ValidationInspector />
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
