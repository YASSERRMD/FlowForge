package graph

import (
	"encoding/json"
	"testing"
)

func TestContextMergeAndLookup(t *testing.T) {
	ctx, err := NewContext(json.RawMessage(`{"invoice":{"amount":120,"vendor":"acme"},"tags":["a","b"]}`))
	if err != nil {
		t.Fatalf("new context: %v", err)
	}
	if ctx.Version != 1 {
		t.Fatalf("version = %d, want 1", ctx.Version)
	}
	if v, ok := ctx.TriggerValue([]string{"invoice", "amount"}); !ok || v.(float64) != 120 {
		t.Fatalf("nested lookup = %v, %v", v, ok)
	}
	if v, ok := ctx.TriggerValue([]string{"tags", "1"}); !ok || v.(string) != "b" {
		t.Fatalf("array lookup = %v, %v", v, ok)
	}
	if _, ok := ctx.TriggerValue([]string{"missing"}); ok {
		t.Fatal("missing key resolved")
	}

	if err := ctx.MergeStep("action-1", json.RawMessage(`{"status":200}`)); err != nil {
		t.Fatalf("merge: %v", err)
	}
	if ctx.Version != 2 {
		t.Fatalf("version after merge = %d, want 2", ctx.Version)
	}
	if v, ok := ctx.StepValue("action-1", []string{"status"}); !ok || v.(float64) != 200 {
		t.Fatalf("step value = %v, %v", v, ok)
	}
	if !ctx.HasStep("action-1") || ctx.HasStep("action-2") {
		t.Fatal("HasStep mismatch")
	}

	// Replacement merge keeps one output and bumps the version.
	if err := ctx.MergeStep("action-1", json.RawMessage(`{"status":201}`)); err != nil {
		t.Fatalf("replace merge: %v", err)
	}
	if ctx.Version != 3 {
		t.Fatalf("version after replace = %d, want 3", ctx.Version)
	}
	if v, _ := ctx.StepValue("action-1", []string{"status"}); v.(float64) != 201 {
		t.Fatalf("step value not replaced: %v", v)
	}
}

func TestContextRejectsBadInputs(t *testing.T) {
	if _, err := NewContext(json.RawMessage(`[1,2]`)); err == nil {
		t.Fatal("array trigger accepted")
	}
	if _, err := NewContext(json.RawMessage(`"text"`)); err == nil {
		t.Fatal("string trigger accepted")
	}
	ctx, _ := NewContext(nil)
	if err := ctx.MergeStep("", json.RawMessage(`{}`)); err == nil {
		t.Fatal("empty node id accepted")
	}
}

func TestContextEmptyTriggerVersion(t *testing.T) {
	ctx, err := NewContext(nil)
	if err != nil {
		t.Fatalf("nil trigger: %v", err)
	}
	if ctx.Version != 1 {
		t.Fatalf("version = %d", ctx.Version)
	}
	if _, ok := ctx.TriggerValue([]string{"anything"}); ok {
		t.Fatal("empty trigger resolved a key")
	}
}
