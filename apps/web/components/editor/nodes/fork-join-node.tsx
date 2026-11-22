import { Handle, Position, type NodeProps } from "@xyflow/react";
import { Split, Merge } from "lucide-react";

import type { FlowNode } from "@/lib/editor/store";
import { NodeChrome } from "./node-chrome";
import { EditableLabel } from "./editable-label";

export const FORK_BRANCHES = ["branch-1", "branch-2", "branch-3"] as const;
export const JOIN_INPUTS = ["in-1", "in-2", "in-3"] as const;

/** Fork: splits execution into parallel branches. */
export function ForkNode({ id, data, selected }: NodeProps<FlowNode>) {
  return (
    <NodeChrome tone="parallel" selected={selected}>
      <Handle type="target" position={Position.Top} id="in" />
      <div className="flex items-center gap-2 font-medium">
        <Split className="h-4 w-4" aria-hidden="true" />
        <EditableLabel nodeId={id} label={data.label} />
      </div>
      <div className="mt-3 flex justify-between gap-2 text-[10px] font-semibold">
        {FORK_BRANCHES.map((b) => (
          <span key={b} aria-hidden="true">
            {b.replace("-", " ")}
          </span>
        ))}
      </div>
      {FORK_BRANCHES.map((branch, i) => (
        <Handle
          key={branch}
          type="source"
          position={Position.Bottom}
          id={branch}
          style={{ left: `${((i + 1) / (FORK_BRANCHES.length + 1)) * 100}%` }}
          aria-label={`Fork output ${branch}`}
        />
      ))}
    </NodeChrome>
  );
}

/** Join: waits for all parallel branches before continuing. */
export function JoinNode({ id, data, selected }: NodeProps<FlowNode>) {
  return (
    <NodeChrome tone="parallel" selected={selected}>
      {JOIN_INPUTS.map((input, i) => (
        <Handle
          key={input}
          type="target"
          position={Position.Top}
          id={input}
          style={{ left: `${((i + 1) / (JOIN_INPUTS.length + 1)) * 100}%` }}
          aria-label={`Join input ${input}`}
        />
      ))}
      <div className="flex items-center gap-2 font-medium">
        <Merge className="h-4 w-4" aria-hidden="true" />
        <EditableLabel nodeId={id} label={data.label} />
      </div>
      <Handle
        type="source"
        position={Position.Bottom}
        id="out"
        aria-label="Join output"
      />
    </NodeChrome>
  );
}
