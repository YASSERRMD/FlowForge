"use client";

import type { ApiClient } from "@/lib/api/client";
import {
  resolveConflictByReload,
  usePersistenceStore,
} from "@/lib/editor/persistence";
import { Button } from "@/components/ui/button";

/**
 * Autosave status line and conflict banner. Conflicts stop autosave and
 * offer an explicit reload — local edits are never silently overwritten
 * and never silently discarded.
 */
export function SaveStatus({ client }: { client: ApiClient }) {
  const status = usePersistenceStore((s) => s.status);
  const lastSavedAt = usePersistenceStore((s) => s.lastSavedAt);
  const errorMessage = usePersistenceStore((s) => s.errorMessage);
  const rowVersion = usePersistenceStore((s) => s.rowVersion);

  if (status === "conflict") {
    return (
      <div
        role="alert"
        className="flex items-center justify-between gap-3 rounded-lg border border-warning bg-warning/10 px-4 py-2 text-sm text-foreground"
      >
        <p>
          <strong>Conflict:</strong> {errorMessage} Your latest edits are still on
          screen but are not being saved.
        </p>
        <Button
          variant="outline"
          size="sm"
          onClick={() => void resolveConflictByReload(client)}
        >
          Reload server version
        </Button>
      </div>
    );
  }

  const label =
    status === "loading"
      ? "Loading draft…"
      : status === "saving"
        ? "Saving…"
        : status === "error"
          ? `Save failed: ${errorMessage}`
          : lastSavedAt
            ? `Saved ${new Date(lastSavedAt).toLocaleTimeString()}`
            : rowVersion !== null
              ? "All changes saved"
              : "Local only — not connected to a draft";

  return (
    <div
      aria-live="polite"
      data-testid="save-status"
      className={
        status === "error"
          ? "rounded-lg border border-error bg-error/10 px-4 py-2 text-sm text-error"
          : "px-1 text-xs text-muted-foreground"
      }
    >
      {label}
    </div>
  );
}
