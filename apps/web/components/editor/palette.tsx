"use client";

import * as React from "react";
import {
  Play,
  Timer,
  GitBranch,
  Split,
  Merge,
  type LucideIcon,
} from "lucide-react";

import { cn } from "@/lib/utils";
import type { NodeKind } from "@/lib/editor/types";
import { NODE_KIND_LABELS, PALETTE_KINDS } from "@/lib/editor/types";

export const DND_MIME = "application/x-flowforge-node";

const KIND_ICONS: Record<NodeKind, LucideIcon> = {
  start: Play,
  end: Play,
  action: Play,
  delay: Timer,
  condition: GitBranch,
  fork: Split,
  join: Merge,
};

export function NodePalette({
  onAdd,
}: {
  onAdd: (kind: NodeKind) => void;
}) {
  return (
    <div
      role="toolbar"
      aria-label="Node palette"
      className="flex flex-wrap gap-2"
    >
      {PALETTE_KINDS.map((kind) => {
        const Icon = KIND_ICONS[kind];
        return (
          <button
            key={kind}
            type="button"
            draggable
            data-node-kind={kind}
            aria-label={`Add ${NODE_KIND_LABELS[kind]} node`}
            title={`Drag onto the canvas or press Enter to add a ${NODE_KIND_LABELS[kind]} node`}
            onDragStart={(event) => {
              event.dataTransfer.setData(DND_MIME, kind);
              event.dataTransfer.effectAllowed = "move";
            }}
            onClick={() => onAdd(kind)}
            className={cn(
              "flex cursor-grab items-center gap-2 rounded-md border border-border bg-card px-3 py-2 text-sm text-card-foreground shadow-sm transition-colors",
              "hover:bg-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring active:cursor-grabbing",
            )}
          >
            <Icon className="h-4 w-4 text-muted-foreground" aria-hidden="true" />
            {NODE_KIND_LABELS[kind]}
          </button>
        );
      })}
    </div>
  );
}
