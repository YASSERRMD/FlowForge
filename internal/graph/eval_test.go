package graph

import (
	"encoding/json"
	"strings"
	"testing"
)

func mustContext(t *testing.T, trigger string, steps map[string]string) *Context {
	t.Helper()
	ctx, err := NewContext(json.RawMessage(trigger))
	if err != nil {
		t.Fatalf("context: %v", err)
	}
	for node, out := range steps {
		if err := ctx.MergeStep(node, json.RawMessage(out)); err != nil {
			t.Fatalf("merge %s: %v", node, err)
		}
	}
	return ctx
}

func TestEvaluateExpression(t *testing.T) {
	ctx := mustContext(t, `{"amount":120,"currency":"USD","active":true,"blocked":false,
		"nested":{"total":42},"list":[10,20,30]}`,
		map[string]string{
			"action-1": `{"status":200,"body":{"total":99}}`,
		})

	cases := []struct {
		expr string
		want bool
	}{
		{`$trigger.amount > 100`, true},
		{`$trigger.amount >= 120`, true},
		{`$trigger.amount < 120`, false},
		{`$trigger.currency == "USD"`, true},
		{`$trigger.currency != "EUR"`, true},
		{`$trigger.active`, true},
		{`not $trigger.blocked`, true},
		{`$trigger.amount > 100 and $trigger.currency == "USD"`, true},
		{`$trigger.amount < 100 or $trigger.active`, true},
		{`not ($trigger.amount < 1 or $trigger.blocked)`, true},
		{`$steps.action-1.output.status == 200`, true},
		{`$steps.action-1.output.body.total == 99`, true},
		{`$steps.action-1.output.body.total > $trigger.nested.total`, true},
		{`$trigger.missing == null`, true},
		{`$trigger.missing != null`, false},
	}
	for _, tc := range cases {
		got, err := EvaluateExpression(tc.expr, ctx)
		if err != nil {
			t.Errorf("%s: unexpected error %v", tc.expr, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%s = %v, want %v", tc.expr, got, tc.want)
		}
	}
}

func TestEvaluateExpressionErrors(t *testing.T) {
	ctx := mustContext(t, `{"amount":"not-a-number","active":true}`, nil)
	cases := []struct {
		expr    string
		snippet string
	}{
		{`$trigger.amount > 10`, "numeric operands"},
		{`$trigger.amount`, "boolean"},
		{`$trigger.active > 1`, "numeric operands"},
		{`$trigger.active + 1`, "unexpected character"},
		{`eval($trigger.amount)`, "unknown identifier"},
	}
	for _, tc := range cases {
		_, err := EvaluateExpression(tc.expr, ctx)
		if err == nil {
			t.Errorf("%s: expected error", tc.expr)
			continue
		}
		if !strings.Contains(err.Error(), tc.snippet) {
			t.Errorf("%s error = %q, want %q", tc.expr, err, tc.snippet)
		}
	}
}

func TestEvaluateExpressionShortCircuits(t *testing.T) {
	ctx := mustContext(t, `{"amount":5}`, nil)
	// The right operand would error on its own; `or` must short-circuit.
	if got, err := EvaluateExpression(`$trigger.amount < 10 or $trigger.missing > 5`, ctx); err != nil || !got {
		t.Fatalf("short-circuit or = %v, %v", got, err)
	}
	if got, err := EvaluateExpression(`$trigger.amount > 10 and $trigger.missing > 5`, ctx); err != nil || got {
		t.Fatalf("short-circuit and = %v, %v", got, err)
	}
}
