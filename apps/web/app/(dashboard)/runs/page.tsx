import { PageHeader } from "@/components/layout/page-header";

export default function RunsPage() {
  return (
    <>
      <PageHeader
        title="Runs"
        description="Inspect execution history and live run status."
      />
      <p className="text-sm text-muted-foreground">
        Run overview UI is under construction. No API integration yet.
      </p>
    </>
  );
}
