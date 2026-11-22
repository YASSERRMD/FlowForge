import { PageHeader } from "@/components/layout/page-header";
import { RunSummaryCards } from "@/components/runs/run-summary-cards";
import { RunTrendChart } from "@/components/runs/run-trend-chart";
import { DEMO_RUN_STATS, DEMO_RUN_TREND } from "@/lib/demo/runs";

export default function RunsPage() {
  return (
    <>
      <PageHeader
        title="Runs"
        description="Inspect execution history and live run status."
      />
      <p
        className="mb-4 rounded-md border border-dashed border-warning bg-warning-muted px-3 py-2 text-sm text-warning-muted-foreground"
        role="note"
      >
        Demo data — run metrics below are static placeholders until the runs
        API ships. No live data is shown.
      </p>
      <div className="flex flex-col gap-4">
        <RunSummaryCards stats={DEMO_RUN_STATS} />
        <RunTrendChart data={DEMO_RUN_TREND} />
      </div>
    </>
  );
}
