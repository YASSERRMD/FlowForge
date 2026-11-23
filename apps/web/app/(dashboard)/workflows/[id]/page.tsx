import { PageHeader } from "@/components/layout/page-header";
import { PersistentEditor } from "@/components/editor/persistent-editor";

export default async function WorkflowEditorPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  return (
    <>
      <PageHeader
        title="Workflow editor"
        description="Edits autosave to the draft with optimistic concurrency; conflicts are shown, never silently overwritten."
      />
      <PersistentEditor workflowId={id} />
    </>
  );
}
