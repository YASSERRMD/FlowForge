import { PageHeader } from "@/components/layout/page-header";
import { TemplateGallery } from "@/components/workflows/template-gallery";

export default function TemplatesPage() {
  return (
    <>
      <PageHeader
        title="Templates"
        description="Start from built-in workflow templates — preview the steps, then create your own editable copy."
      />
      <TemplateGallery />
    </>
  );
}
