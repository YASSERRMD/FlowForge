import { PageHeader } from "@/components/layout/page-header";
import { WorkflowEditor } from "@/components/editor/workflow-editor";

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
        description={`Editing draft ${id} (local canvas state only — persistence ships in a later phase).`}
      />
      <WorkflowEditor />
    </>
  );
}
