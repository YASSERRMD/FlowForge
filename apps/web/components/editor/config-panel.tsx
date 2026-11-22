"use client";

import * as React from "react";

import { useEditorStore } from "@/lib/editor/store";
import type { FlowNode } from "@/lib/editor/store";
import {
  CONFIG_FIELDS,
  validateField,
  type ConfigField,
} from "@/lib/editor/config-schema";
import { NODE_KIND_LABELS } from "@/lib/editor/types";
import { FormField } from "@/components/ui/form-field";
import { Input } from "@/components/ui/input";

function fieldError(field: ConfigField, value: unknown): string | null {
  return validateField(field, value);
}

function ConfigInput({
  node,
  field,
}: {
  node: FlowNode;
  field: ConfigField;
}) {
  const updateNodeConfig = useEditorStore((s) => s.updateNodeConfig);
  const value = node.data.config?.[field.key];
  const text = value === undefined || value === null ? "" : String(value);
  const [touched, setTouched] = React.useState(false);
  const error = touched ? fieldError(field, text === "" ? undefined : value) : null;

  const commit = (raw: string) => {
    setTouched(true);
    let next: unknown = raw;
    if (field.type === "number") {
      next = raw === "" ? undefined : Number(raw);
    } else if (field.key === "params") {
      next = raw;
    }
    updateNodeConfig(node.id, { [field.key]: next });
  };

  return (
    <FormField label={field.label} error={error ?? undefined} hint={field.help}>
      {(props) =>
        field.type === "select" ? (
          <select
            {...props}
            value={text}
            onChange={(e) => commit(e.target.value)}
            onBlur={() => setTouched(true)}
            className="nodrag h-9 w-full rounded-md border border-input bg-background px-3 text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-ring"
          >
            <option value="">Select…</option>
            {field.options?.map((o) => (
              <option key={o.value} value={o.value}>
                {o.label}
              </option>
            ))}
          </select>
        ) : (
          <Input
            {...props}
            value={text}
            placeholder={field.placeholder}
            inputMode={field.type === "number" ? "numeric" : undefined}
            onChange={(e) => commit(e.target.value)}
            onBlur={() => setTouched(true)}
            className="nodrag"
          />
        )
      }
    </FormField>
  );
}

/**
 * Side panel with schema-driven configuration forms for the selected node.
 * Hidden for kinds without config (start/end) and when nothing is selected.
 */
export function NodeConfigPanel() {
  const node = useEditorStore((s) => s.nodes.find((n) => n.selected));
  const fields = node ? CONFIG_FIELDS[node.data.kind] : undefined;

  if (!node || !fields || fields.length === 0) return null;

  return (
    <aside
      aria-label="Node configuration"
      className="flex w-72 shrink-0 flex-col gap-3 rounded-lg border border-border bg-card p-4"
    >
      <div>
        <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
          {NODE_KIND_LABELS[node.data.kind]}
        </p>
        <h2 className="text-sm font-semibold text-foreground">{node.data.label}</h2>
      </div>
      {fields.map((field) => (
        <ConfigInput key={`${node.id}:${field.key}`} node={node} field={field} />
      ))}
    </aside>
  );
}
