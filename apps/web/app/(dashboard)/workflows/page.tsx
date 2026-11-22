import { PageHeader } from "@/components/layout/page-header";
import { WorkflowListing } from "@/components/workflows/workflow-listing";

export default function WorkflowsPage() {
  return (
    <>
      <PageHeader
        title="Workflows"
        description="Design, version, and monitor your automation workflows."
      />
      <WorkflowListing />
    </>
  );
}
