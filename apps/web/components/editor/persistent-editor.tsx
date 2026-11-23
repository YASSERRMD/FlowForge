"use client";

import * as React from "react";

import { createBrowserClient } from "@/lib/api/client";
import {
  loadWorkflow,
  startAutosave,
  stopAutosave,
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

  React.useEffect(() => {
    loadWorkflow(client, workflowId).catch(() => {
      // The editor stays usable with its local state; SaveStatus shows
      // the load error and autosave keeps retrying on the next edit.
    });
    const stop = startAutosave(client);
    return () => {
      stop();
      stopAutosave();
    };
  }, [client, workflowId]);

  return (
    <div className="flex flex-col gap-3">
      <SaveStatus client={client} />
      <WorkflowEditor />
    </div>
  );
}
