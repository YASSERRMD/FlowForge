import { Handle, Position, type NodeProps } from "@xyflow/react";
import { Zap, Timer } from "lucide-react";

import type { FlowNode } from "@/lib/editor/store";
import { NodeChrome } from "./node-chrome";
import { EditableLabel } from "./editable-label";

function BaseStepNode({
  id,
  data,
  selected,
  icon: Icon,
  kindLabel,
}: NodeProps<FlowNode> & { icon: typeof Zap; kindLabel: string }) {
  return (
    <NodeChrome
      tone="default"
      selected={selected}
      className={selected ? "border-ring" : undefined}
    >
      <Handle type="target" position={Position.Top} id="in" />
      <div className="flex items-center gap-2">
        <Icon className="h-4 w-4 text-primary" aria-hidden="true" />
        <span className="sr-only">{kindLabel} node:</span>
        <EditableLabel nodeId={id} label={data.label} />
      </div>
      <Handle type="source" position={Position.Bottom} id="out" />
    </NodeChrome>
  );
}

export function ActionNode(props: NodeProps<FlowNode>) {
  return <BaseStepNode {...props} icon={Zap} kindLabel="Action" />;
}

export function DelayNode(props: NodeProps<FlowNode>) {
  return <BaseStepNode {...props} icon={Timer} kindLabel="Delay" />;
}
