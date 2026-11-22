import type { ReactNode } from "react";

import { cn } from "@/lib/utils";

/** Shared visual frame for all custom nodes. */
export function NodeChrome({
  selected,
  tone,
  children,
  className,
}: {
  selected?: boolean;
  tone: "start" | "end" | "default" | "condition" | "parallel";
  children: ReactNode;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "min-w-32 rounded-lg border px-3 py-2 text-sm shadow-sm transition-colors",
        tone === "start" &&
          "border-success bg-success-muted text-success-muted-foreground",
        tone === "end" &&
          "border-error bg-error-muted text-error-muted-foreground",
        tone === "default" && "bg-card text-card-foreground",
        tone === "condition" &&
          "border-warning bg-warning-muted text-warning-muted-foreground",
        tone === "parallel" &&
          "border-info bg-info-muted text-info-muted-foreground",
        selected ? "border-ring ring-2 ring-ring" : "border-border",
        className,
      )}
    >
      {children}
    </div>
  );
}
