import {
  Activity,
  CheckCircle2,
  Clock,
  Play,
  XCircle,
} from "lucide-react";

import type { RunStats } from "@/lib/demo/runs";

function SummaryCard({
  title,
  value,
  icon: Icon,
  tone,
}: {
  title: string;
  value: string;
  icon: typeof Activity;
  tone: "default" | "success" | "error" | "info";
}) {
  const toneClass = {
    default: "text-foreground",
    success: "text-success",
    error: "text-error",
    info: "text-info",
  }[tone];

  return (
    <div className="rounded-lg border border-border bg-card p-4 text-card-foreground">
      <div className="flex items-center justify-between">
        <p className="text-sm text-muted-foreground">{title}</p>
        <Icon className={`h-4 w-4 ${toneClass}`} aria-hidden="true" />
      </div>
      <p className={`mt-2 text-2xl font-semibold ${toneClass}`}>{value}</p>
    </div>
  );
}

export function RunSummaryCards({ stats }: { stats: RunStats }) {
  return (
    <div
      className="grid grid-cols-2 gap-4 lg:grid-cols-5"
      aria-label="Run status summary"
    >
      <SummaryCard
        title="Total runs"
        value={stats.total.toLocaleString()}
        icon={Activity}
        tone="default"
      />
      <SummaryCard
        title="Succeeded"
        value={stats.succeeded.toLocaleString()}
        icon={CheckCircle2}
        tone="success"
      />
      <SummaryCard
        title="Failed"
        value={stats.failed.toLocaleString()}
        icon={XCircle}
        tone="error"
      />
      <SummaryCard
        title="Running"
        value={stats.running.toLocaleString()}
        icon={Play}
        tone="info"
      />
      <SummaryCard
        title="Avg duration"
        value={`${stats.avgDurationSeconds}s`}
        icon={Clock}
        tone="default"
      />
    </div>
  );
}
