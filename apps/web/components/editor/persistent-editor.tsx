"use client";

import * as React from "react";

import { createBrowserClient } from "@/lib/api/client";
import {
  loadWorkflow,
  startAutosave,
  stopAutosave,
  usePersistenceStore,
} from "@/lib/editor/persistence";
import { SaveStatus } from "./save-status";
import { WorkflowEditor } from "./workflow-editor";

/**
 * Editor bound to a server draft: loads the graph on mount and autosaves
 * with optimistic concurrency. The dev identity header is development-only
 * (APP_ENV=development, AUTH_DEV_MODE=true on the API).
 */
export function PersistentEditor({ workflowId }: { workflowId: string }) {
  const client = React.useMemo(
    () => createBrowserClient(process.env.NEXT_PUBLIC_DEV_USER ?? "local-dev"),
    [],
  );
  const status = usePersistenceStore((s) => s.status);
  const errorMessage = usePersistenceStore((s) => s.errorMessage);

  React.useEffect(() => {
    loadWorkflow(client, workflowId).catch(() => {
      /* error state is rendered below */
    });
    const stop = startAutosave(client);
    return () => {
      stop();
      stopAutosave();
    };
  }, [client, workflowId]);

  if (status === "error" && errorMessage && usePersistenceStore.getState().rowVersion === null) {
    return (
      <div role="alert" className="rounded-lg border border-error bg-error-muted p-4 text-sm text-error-muted-foreground">
        Could not load this workflow: {errorMessage}
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-3">
      <SaveStatus client={client} />
      <WorkflowEditor />
    </div>
  );
}
