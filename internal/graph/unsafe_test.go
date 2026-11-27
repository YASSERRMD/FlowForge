package graph

import (
	"strings"
	"testing"
)

// TestRejectUnsafeExpressions pins the security posture of the condition
// language: no code execution, no prototype paths, no statement chaining.
func TestRejectUnsafeExpressions(t *testing.T) {
	unsafe := []struct {
		expr    string
		snippet string
	}{
		{`eval("1+1")`, "unknown identifier"},
		{`require("child_process")`, "unknown identifier"},
		{`process.exit(1)`, "unknown identifier"},
		{`$trigger.a; $trigger.b`, "unexpected character"},
		{"$trigger.a == `rm -rf`", "unexpected character"},
		{`$trigger.__proto__ == 1`, "unsafe reference"},
		{`$trigger.constructor == 1`, "unsafe reference"},
		{`$trigger.a.prototype == 1`, "unsafe reference"},
	}
	for _, tc := range unsafe {
		msg := ValidateExpression(tc.expr)
		if msg == "" {
			t.Errorf("%q was accepted but must be rejected", tc.expr)
			continue
		}
		if !strings.Contains(msg, tc.snippet) {
			t.Errorf("%q error = %q, want snippet %q", tc.expr, msg, tc.snippet)
		}
	}
}

func TestSafePathsStillWork(t *testing.T) {
	for _, expr := range []string{
		`$trigger.constructor_id == 1`,
		`$steps.action-1.output.__typename == "x"`,
		`$trigger.a.b_c == "value"`,
	} {
		if msg := ValidateExpression(expr); msg != "" {
			t.Errorf("%q should be valid, got %q", expr, msg)
		}
	}
}
