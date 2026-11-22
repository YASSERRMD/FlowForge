import { Handle, Position, type NodeProps } from "@xyflow/react";
import { Play } from "lucide-react";

import type { FlowNode } from "@/lib/editor/store";
import { NodeChrome } from "./node-chrome";

/** Start node: exactly one outgoing handle, no incoming connections. */
export function StartNode({ data, selected }: NodeProps<FlowNode>) {
  return (
    <NodeChrome tone="start" selected={selected}>
      <div className="flex items-center gap-2 font-medium">
        <Play className="h-4 w-4" aria-hidden="true" />
        {data.label}
      </div>
      <Handle
        type="source"
        position={Position.Bottom}
        id="out"
        aria-label="Start output"
      />
    </NodeChrome>
  );
}
