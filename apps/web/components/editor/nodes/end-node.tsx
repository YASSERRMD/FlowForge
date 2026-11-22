import { Handle, Position, type NodeProps } from "@xyflow/react";
import { Flag } from "lucide-react";

import type { FlowNode } from "@/lib/editor/store";
import { NodeChrome } from "./node-chrome";

/** End node: incoming-only; it can never originate an edge. */
export function EndNode({ data, selected }: NodeProps<FlowNode>) {
  return (
    <NodeChrome tone="end" selected={selected}>
      <Handle
        type="target"
        position={Position.Top}
        id="in"
        aria-label="End input"
      />
      <div className="flex items-center gap-2 font-medium">
        <Flag className="h-4 w-4" aria-hidden="true" />
        {data.label}
      </div>
    </NodeChrome>
  );
}
