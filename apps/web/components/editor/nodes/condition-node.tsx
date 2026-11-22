import { Handle, Position, type NodeProps } from "@xyflow/react";
import { GitBranch } from "lucide-react";

import type { FlowNode } from "@/lib/editor/store";
import { NodeChrome } from "./node-chrome";
import { EditableLabel } from "./editable-label";

/** Condition node: one input, explicitly labeled true/false outputs. */
export function ConditionNode({ id, data, selected }: NodeProps<FlowNode>) {
  return (
    <NodeChrome tone="condition" selected={selected}>
      <Handle type="target" position={Position.Top} id="in" />
      <div className="flex items-center gap-2 font-medium">
        <GitBranch className="h-4 w-4" aria-hidden="true" />
        <EditableLabel nodeId={id} label={data.label} />
      </div>
      <div className="mt-3 flex justify-between text-xs font-semibold">
        <span aria-hidden="true">true</span>
        <span aria-hidden="true">false</span>
      </div>
      <Handle
        type="source"
        position={Position.Bottom}
        id="true"
        style={{ left: "25%" }}
        aria-label="True branch output"
      />
      <Handle
        type="source"
        position={Position.Bottom}
        id="false"
        style={{ left: "75%" }}
        aria-label="False branch output"
      />
    </NodeChrome>
  );
}
