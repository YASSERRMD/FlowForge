import { describe, expect, it } from "vitest";

import { validateExpression } from "./expr";

// Mirrors internal/graph/expr_test.go — both sides must agree.
describe("validateExpression", () => {
  it.each([
    `$trigger.amount > 100`,
    `$trigger.currency == "USD"`,
    `$trigger.amount > 100 and $trigger.currency == "USD"`,
    `$trigger.active`,
    `$trigger.active == true`,
    `not $trigger.blocked`,
    `not ($trigger.a < 1 or $trigger.b < 2)`,
    `$steps.action-1.output.status == 200`,
    `$steps.action-1.output.body.total >= $trigger.min_total`,
    `($trigger.a == 1 and $trigger.b == 2) or $trigger.c == null`,
    `$trigger.threshold <= -3.5`,
  ])("accepts %s", (src) => {
    expect(validateExpression(src)).toBe("");
  });

  it.each([
    ["foo == 1", "unknown identifier"],
    ["$foo.bar == 1", "unknown reference root"],
    ["$steps.action-1.status == 200", "$steps.<nodeId>.output"],
    ['$trigger.amount > "100"', "numeric operands"],
    ["$trigger.amount >", "missing value"],
    ["($trigger.a == 1", "closing parenthesis"],
    ["$trigger.a == 1 )", "trailing"],
    ["$trigger.a == 1 and", "unexpected end"],
    ["$trigger.a..b == 1", "invalid path segment"],
    ["123", "comparison operator"],
  ])("rejects %s mentioning %s", (src, snippet) => {
    const msg = validateExpression(src);
    expect(msg).not.toBe("");
    expect(msg).toContain(snippet);
  });

  it("reports 1-based positions", () => {
    expect(validateExpression("$trigger.a == 1 and bogus == 2")).toContain(
      "position 21",
    );
  });
});
