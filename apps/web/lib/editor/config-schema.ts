/**
 * Schema-driven node configuration: per-kind field descriptors that drive
 * both the config panel forms and their validation.
 * See docs/contracts/graph.md for reserved config keys.
 */
import type { GraphNodeConfig, GraphNodeKind } from "@/lib/contracts/graph";

export type FieldType = "string" | "number" | "select" | "expression";

export interface ConfigField {
  key: string;
  label: string;
  type: FieldType;
  required?: boolean;
  placeholder?: string;
  help?: string;
  options?: { value: string; label: string }[];
  /** type "number": inclusive bounds. */
  min?: number;
  max?: number;
}

/** Kinds that have editable configuration. start/end have none. */
export const CONFIG_FIELDS: Partial<Record<GraphNodeKind, ConfigField[]>> = {
  action: [
    {
      key: "actionType",
      label: "Action type",
      type: "select",
      required: true,
      options: [
        { value: "http.request", label: "HTTP request" },
        { value: "email.send", label: "Send email" },
        { value: "transform.map", label: "Transform data" },
        { value: "queue.publish", label: "Publish to queue" },
      ],
      help: "The registered action this node runs.",
    },
    {
      key: "params",
      label: "Parameters (JSON)",
      type: "string",
      placeholder: '{ "url": "https://…" }',
      help: "Free-form JSON object passed to the action.",
    },
  ],
  delay: [
    {
      key: "durationMs",
      label: "Duration (ms)",
      type: "number",
      required: true,
      min: 1,
      max: 31_536_000_000, // 1 year
      placeholder: "60000",
      help: "How long the run waits before continuing.",
    },
  ],
  condition: [
    {
      key: "expression",
      label: "Expression",
      type: "expression",
      required: true,
      placeholder: '$trigger.amount > 100 and $trigger.currency == "USD"',
      help: "Typed comparison over trigger/step data. See docs/contracts/conditions.md.",
    },
  ],
  fork: [
    {
      key: "branchCount",
      label: "Branches",
      type: "number",
      required: true,
      min: 2,
      max: 16,
      placeholder: "2",
      help: "Number of parallel branches this fork starts.",
    },
  ],
  join: [
    {
      key: "branchCount",
      label: "Branches",
      type: "number",
      required: true,
      min: 2,
      max: 16,
      placeholder: "2",
      help: "Number of parallel branches this join waits for.",
    },
  ],
};

export interface ConfigIssue {
  key: string;
  message: string;
}

/**
 * Validates one config value against its field descriptor.
 * Returns null when valid, otherwise a human-readable message.
 */
export function validateField(field: ConfigField, value: unknown): string | null {
  const empty = value === undefined || value === null || value === "";
  if (empty) return field.required ? `${field.label} is required.` : null;

  switch (field.type) {
    case "number": {
      const n = typeof value === "string" ? Number(value) : value;
      if (typeof n !== "number" || !Number.isFinite(n) || !Number.isInteger(n))
        return `${field.label} must be an integer.`;
      if (field.min !== undefined && n < field.min)
        return `${field.label} must be at least ${field.min}.`;
      if (field.max !== undefined && n > field.max)
        return `${field.label} must be at most ${field.max}.`;
      return null;
    }
    case "select": {
      if (field.options && !field.options.some((o) => o.value === value))
        return `${field.label} must be one of the listed options.`;
      return null;
    }
    case "expression": {
      if (typeof value !== "string" || value.trim() === "")
        return `${field.label} must be a non-empty expression.`;
      // Full grammar validation lives in lib/editor/expr.ts (phase 06.05);
      // the panel performs that separately so this layer stays kind-generic.
      return null;
    }
    case "string": {
      if (field.key === "params" && typeof value === "string" && value.trim() !== "") {
        try {
          const parsed: unknown = JSON.parse(value);
          if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed))
            return `${field.label} must be a JSON object.`;
        } catch {
          return `${field.label} must be valid JSON.`;
        }
      }
      return null;
    }
  }
}

/** Validates every field of a node's config. Unknown keys are not checked. */
export function validateConfig(
  kind: GraphNodeKind,
  config: GraphNodeConfig | undefined,
): ConfigIssue[] {
  const fields = CONFIG_FIELDS[kind] ?? [];
  const issues: ConfigIssue[] = [];
  for (const field of fields) {
    const message = validateField(field, config?.[field.key]);
    if (message) issues.push({ key: field.key, message });
  }
  return issues;
}
