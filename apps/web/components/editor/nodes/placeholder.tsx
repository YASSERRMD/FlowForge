import { Handle, Position, type NodeProps } from "@xyflow/react";

import type { FlowNode } from "@/lib/editor/store";

// Temporary rendering until dedicated node components land (05.03+).
export function PlaceholderNode({ data, selected }: NodeProps<FlowNode>) {
  return (
    <div
      className={`rounded-md border bg-card px-3 py-2 text-sm text-card-foreground ${
        selected ? "border-ring" : "border-border"
      }`}
    >
      <Handle type="target" position={Position.Top} />
      {data.label}
      <Handle type="source" position={Position.Bottom} />
    </div>
  );
}
