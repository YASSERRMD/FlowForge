export type NodeKind =
  | "start"
  | "end"
  | "action"
  | "delay"
  | "condition"
  | "fork"
  | "join";

export interface FlowNodeData extends Record<string, unknown> {
  label: string;
  kind: NodeKind;
}

export const NODE_KIND_LABELS: Record<NodeKind, string> = {
  start: "Start",
  end: "End",
  action: "Action",
  delay: "Delay",
  condition: "Condition",
  fork: "Fork",
  join: "Join",
};

/** Editable node kinds (start/end have fixed semantics and labels). */
export const PALETTE_KINDS: NodeKind[] = [
  "action",
  "delay",
  "condition",
  "fork",
  "join",
];
