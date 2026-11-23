import { describe, expect, it } from "vitest";

import {
  CONFIG_FIELDS,
  validateConfig,
  validateField,
} from "./config-schema";

describe("validateField", () => {
  const duration = CONFIG_FIELDS.delay![0];

  it("requires required fields", () => {
    expect(validateField(duration, undefined)).toMatch(/required/);
    expect(validateField(duration, "")).toMatch(/required/);
  });

  it("accepts optional fields when empty", () => {
    const params = CONFIG_FIELDS.action!.find((f) => f.key === "params")!;
    expect(validateField(params, undefined)).toBeNull();
  });

  it("validates integer bounds", () => {
    expect(validateField(duration, 0)).toMatch(/at least 1/);
    expect(validateField(duration, 1.5)).toMatch(/integer/);
    expect(validateField(duration, "abc")).toMatch(/integer/);
    expect(validateField(duration, 60000)).toBeNull();
    expect(validateField(duration, "60000")).toBeNull();
  });

  it("validates select options", () => {
    const actionType = CONFIG_FIELDS.action![0];
    expect(validateField(actionType, "http.request")).toBeNull();
    expect(validateField(actionType, "nope")).toMatch(/one of the listed/);
  });

  it("validates params as a JSON object", () => {
    const params = CONFIG_FIELDS.action!.find((f) => f.key === "params")!;
    expect(validateField(params, '{"url":"x"}')).toBeNull();
    expect(validateField(params, "not json")).toMatch(/valid JSON/);
    expect(validateField(params, "[1,2]")).toMatch(/JSON object/);
  });

  it("rejects empty expressions", () => {
    const expr = CONFIG_FIELDS.condition![0];
    expect(validateField(expr, "   ")).toMatch(/non-empty/);
    expect(validateField(expr, "$trigger.a > 1")).toBeNull();
  });
});

describe("validateConfig", () => {
  it("reports all missing required fields for a kind", () => {
    const issues = validateConfig("delay", {});
    expect(issues).toHaveLength(1);
    expect(issues[0].key).toBe("durationMs");
  });

  it("ignores unknown config keys", () => {
    expect(validateConfig("delay", { durationMs: 5, futureKey: "x" })).toHaveLength(0);
  });

  it("returns no issues for kinds without config", () => {
    expect(validateConfig("start", undefined)).toHaveLength(0);
  });

  it("accepts a valid fork branch count", () => {
    expect(validateConfig("fork", { branchCount: 3 })).toHaveLength(0);
    expect(validateConfig("fork", { branchCount: 1 })[0].message).toMatch(/at least 2/);
  });
});
