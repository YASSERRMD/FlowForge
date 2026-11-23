"use client";

import * as React from "react";
import { useRouter } from "next/navigation";
import { toast } from "sonner";

import {
  ApiError,
  createBrowserClient,
  type Template,
} from "@/lib/api/client";
import type { WorkflowGraph } from "@/lib/contracts/graph";
import { NODE_KIND_LABELS, type NodeKind } from "@/lib/editor/types";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

function graphPreview(graph: unknown): { kind: NodeKind; label: string }[] {
  const doc = graph as WorkflowGraph;
  return (doc?.nodes ?? [])
    .filter((n) => n.kind !== "start" && n.kind !== "end")
    .map((n) => ({ kind: n.kind, label: n.label }));
}

function TemplateCard({
  template,
  onUse,
  busy,
}: {
  template: Template;
  onUse: (template: Template) => void;
  busy: boolean;
}) {
  const [expanded, setExpanded] = React.useState(false);
  const steps = graphPreview(template.graph);

  return (
    <article className="flex flex-col gap-3 rounded-lg border border-border bg-card p-4">
      <div className="flex items-start justify-between gap-2">
        <div>
          <h3 className="text-sm font-semibold text-foreground">{template.name}</h3>
          <p className="mt-1 text-xs text-muted-foreground">{template.description}</p>
        </div>
        <Badge variant="info">{template.source}</Badge>
      </div>

      <div className="flex flex-wrap gap-1">
        {template.tags.map((tag) => (
          <Badge key={tag} variant="outline">{tag}</Badge>
        ))}
      </div>

      <button
        type="button"
        onClick={() => setExpanded((v) => !v)}
        aria-expanded={expanded}
        className="text-left text-xs font-medium text-primary hover:underline"
      >
        {expanded ? "Hide preview" : `Preview ${steps.length} steps`}
      </button>
      {expanded && (
        <ol className="flex flex-col gap-1 rounded-md bg-muted p-2 text-xs">
          {steps.map((step, i) => (
            <li key={i} className="flex items-center gap-2">
              <Badge variant="secondary">{NODE_KIND_LABELS[step.kind]}</Badge>
              <span className="text-foreground">{step.label}</span>
            </li>
          ))}
        </ol>
      )}

      <div className="mt-auto flex gap-2">
        <Button size="sm" disabled={busy} onClick={() => onUse(template)}>
          {busy ? "Creating…" : "Use template"}
        </Button>
      </div>
    </article>
  );
}

/** Searchable gallery of built-in starter templates with preview and copy. */
export function TemplateGallery() {
  const client = React.useMemo(
    () => createBrowserClient(process.env.NEXT_PUBLIC_DEV_USER ?? "local-dev"),
    [],
  );
  const router = useRouter();
  const [templates, setTemplates] = React.useState<Template[] | null>(null);
  const [error, setError] = React.useState<string | null>(null);
  const [query, setQuery] = React.useState("");
  const [busyId, setBusyId] = React.useState<string | null>(null);

  React.useEffect(() => {
    client
      .listTemplates()
      .then(setTemplates)
      .catch((err: unknown) =>
        setError(err instanceof Error ? err.message : "failed to load templates"),
      );
  }, [client]);

  const use = async (template: Template) => {
    setBusyId(template.id);
    try {
      const { workflow } = await client.createWorkflow({
        name: template.name,
        templateId: template.id,
      });
      toast.success("Workflow created from template");
      router.push(`/workflows/${workflow.id}`);
    } catch (err) {
      toast.error("Could not create workflow", {
        description: err instanceof ApiError ? err.message : "unknown error",
      });
      setBusyId(null);
    }
  };

  if (error) {
    return (
      <div role="alert" className="rounded-lg border border-error bg-error-muted p-4 text-sm text-error-muted-foreground">
        Could not load templates: {error}
      </div>
    );
  }
  if (templates === null) {
    return <p className="text-sm text-muted-foreground">Loading templates…</p>;
  }

  const q = query.trim().toLowerCase();
  const filtered = q
    ? templates.filter(
        (t) =>
          t.name.toLowerCase().includes(q) ||
          t.description.toLowerCase().includes(q) ||
          t.tags.some((tag) => tag.toLowerCase().includes(q)),
      )
    : templates;

  return (
    <div className="flex flex-col gap-4">
      <div className="max-w-sm">
        <Input
          type="search"
          placeholder="Search templates by name, tag or description…"
          aria-label="Search templates"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
      </div>
      {filtered.length === 0 ? (
        <p className="text-sm text-muted-foreground">No templates match “{query}”.</p>
      ) : (
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          {filtered.map((template) => (
            <TemplateCard
              key={template.id}
              template={template}
              onUse={use}
              busy={busyId === template.id}
            />
          ))}
        </div>
      )}
    </div>
  );
}
