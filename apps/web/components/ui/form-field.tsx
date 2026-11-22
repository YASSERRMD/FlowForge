import * as React from "react";

import { cn } from "@/lib/utils";
import { Label } from "@/components/ui/label";

function useFieldId() {
  return `field-${React.useId().replace(/[^a-zA-Z0-9-]/g, "")}`;
}

/**
 * Accessible form field wrapper: wires label, input and error message
 * together with htmlFor / aria-describedby / aria-invalid.
 */
export function FormField({
  label,
  error,
  hint,
  children,
  className,
}: {
  label: string;
  error?: string;
  hint?: string;
  children: (props: {
    id: string;
    "aria-invalid"?: boolean;
    "aria-describedby"?: string;
  }) => React.ReactNode;
  className?: string;
}) {
  const id = useFieldId();
  const errorId = `${id}-error`;
  const hintId = `${id}-hint`;
  const describedBy = [error ? errorId : null, hint ? hintId : null]
    .filter(Boolean)
    .join(" ");

  return (
    <div className={cn("flex flex-col gap-1.5", className)}>
      <Label htmlFor={id}>{label}</Label>
      {children({
        id,
        "aria-invalid": error ? true : undefined,
        "aria-describedby": describedBy || undefined,
      })}
      {hint && !error ? (
        <p id={hintId} className="text-xs text-muted-foreground">
          {hint}
        </p>
      ) : null}
      {error ? (
        <p id={errorId} role="alert" className="text-xs text-error">
          {error}
        </p>
      ) : null}
    </div>
  );
}
