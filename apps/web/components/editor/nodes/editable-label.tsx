"use client";

import * as React from "react";

import { useEditorStore } from "@/lib/editor/store";

function commitLabel(nodeId: string, draft: string, original: string) {
  const trimmed = draft.trim();
  useEditorStore
    .getState()
    .updateNodeLabel(nodeId, trimmed === "" ? original : trimmed);
}

/** Double-click-to-edit node label. Commits on Enter/blur, cancels on Esc. */
export function EditableLabel({
  nodeId,
  label,
}: {
  nodeId: string;
  label: string;
}) {
  const [editing, setEditing] = React.useState(false);
  const [draft, setDraft] = React.useState(label);
  const inputRef = React.useRef<HTMLInputElement>(null);

  React.useEffect(() => {
    if (editing) inputRef.current?.select();
  }, [editing]);

  if (!editing) {
    return (
      <span
        onDoubleClick={() => {
          setDraft(label);
          setEditing(true);
        }}
        title="Double-click to rename"
      >
        {label}
      </span>
    );
  }

  return (
    <input
      ref={inputRef}
      value={draft}
      aria-label="Node label"
      className="nodrag w-full rounded border border-ring bg-background px-1 py-0.5 text-sm text-foreground focus:outline-none"
      onChange={(event) => setDraft(event.target.value)}
      onKeyDown={(event) => {
        if (event.key === "Enter") {
          commitLabel(nodeId, draft, label);
          setEditing(false);
        }
        if (event.key === "Escape") setEditing(false);
      }}
      onBlur={() => {
        commitLabel(nodeId, draft, label);
        setEditing(false);
      }}
    />
  );
}
