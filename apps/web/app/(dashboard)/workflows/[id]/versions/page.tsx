import Link from "next/link";

import { PageHeader } from "@/components/layout/page-header";
import { VersionHistory } from "@/components/workflows/version-history";

export default async function WorkflowVersionsPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  return (
    <>
      <PageHeader
        title="Version history"
        description="Immutable snapshots for this workflow. Select two versions to compare them."
      />
      <p className="mb-4 text-sm">
        <Link className="text-primary underline" href={`/workflows/${id}`}>
          ← Back to the editor
        </Link>
      </p>
      <VersionHistory workflowId={id} />
    </>
  );
}
