import { Badge } from "@/components/ui/badge";
import type { WorkflowStatus } from "@/lib/demo/workflows";

const STATUS_META: Record<
  WorkflowStatus,
  { label: string; variant: "success" | "secondary" | "warning" | "error" }
> = {
  active: { label: "Active", variant: "success" },
  draft: { label: "Draft", variant: "secondary" },
  paused: { label: "Paused", variant: "warning" },
  error: { label: "Error", variant: "error" },
};

export function WorkflowStatusChip({ status }: { status: WorkflowStatus }) {
  const meta = STATUS_META[status];
  return <Badge variant={meta.variant}>{meta.label}</Badge>;
}
