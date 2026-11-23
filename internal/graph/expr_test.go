package graph

import (
	"strings"
	"testing"
)

func TestValidateExpressionValid(t *testing.T) {
	valid := []string{
		`$trigger.amount > 100`,
		`$trigger.currency == "USD"`,
		`$trigger.amount > 100 and $trigger.currency == "USD"`,
		`$trigger.active`,
		`$trigger.active == true`,
		`not $trigger.blocked`,
		`not ($trigger.a < 1 or $trigger.b < 2)`,
		`$steps.action-1.output.status == 200`,
		`$steps.action-1.output.body.total >= $trigger.min_total`,
		`$trigger.name == "it\'s"`,
		`($trigger.a == 1 and $trigger.b == 2) or $trigger.c == null`,
		`$trigger.threshold <= -3.5`,
	}
	for _, src := range valid {
		if msg := ValidateExpression(src); msg != "" {
			t.Errorf("expected %q to be valid, got %q", src, msg)
		}
	}
}

func TestValidateExpressionInvalid(t *testing.T) {
	invalid := []struct {
		src     string
		snippet string
	}{
		{``, "empty"},
		{`foo == 1`, "unknown identifier"},
		{`$foo.bar == 1`, "unknown reference root"},
		{`$trigger == 1`, "unknown reference root"},
		{`$steps.action-1.status == 200`, "expected $steps.<nodeId>.output"},
		{`$trigger.amount > "100"`, "numeric operands"},
		{`$trigger.amount >`, "missing value"},
		{`$trigger.amount == `, "missing value"},
		{`$trigger.amount = 1`, "expected"},
		{`($trigger.a == 1`, "closing parenthesis"},
		{`$trigger.a == 1)`, "trailing"},
		{`$trigger.a == "unterminated`, "unterminated"},
		{`$trigger.a == 1 xor $trigger.b == 2`, "unknown identifier"},
		{`$trigger.a == 1 and`, "unexpected end"},
		{`eval("x")`, "unknown identifier"},
		{`$trigger.a..b == 1`, "invalid path segment"},
		{`123`, "comparison operator"},
	}
	for _, tc := range invalid {
		msg := ValidateExpression(tc.src)
		if msg == "" {
			t.Errorf("expected %q to be invalid", tc.src)
			continue
		}
		if !strings.Contains(msg, tc.snippet) {
			t.Errorf("error for %q = %q, want snippet %q", tc.src, msg, tc.snippet)
		}
	}
}

func TestValidateExpressionErrorPosition(t *testing.T) {
	msg := ValidateExpression(`$trigger.a == 1 and bogus == 2`)
	if !strings.Contains(msg, "position 21") {
		t.Errorf("expected position 21 in %q", msg)
	}
}
