package engine

import (
	"encoding/json"
	"testing"

	"github.com/YASSERRMD/FlowForge/internal/graph"
)

func testContext(t *testing.T, trigger string, steps map[string]string) *graph.Context {
	t.Helper()
	rc, err := graph.NewContext(json.RawMessage(trigger))
	if err != nil {
		t.Fatalf("context: %v", err)
	}
	for node, out := range steps {
		if err := rc.MergeStep(node, json.RawMessage(out)); err != nil {
			t.Fatalf("merge: %v", err)
		}
	}
	return rc
}

func TestResolveParams(t *testing.T) {
	rc := testContext(t, `{"invoice_id":"inv-9","amount":42,"meta":{"deep":true}}`,
		map[string]string{"action-1": `{"status":"ok","total":7}`})

	raw := json.RawMessage(`{
		"invoiceId":"$trigger.invoice_id",
		"amount":"$trigger.amount",
		"deep":"$trigger.meta.deep",
		"total":"$steps.action-1.output.total",
		"literal":"please keep $trigger.invoice_id inline",
		"nested":{"list":["$trigger.invoice_id",123,true]},
		"missing":"$trigger.nope"
	}`)
	out, err := resolveParams(raw, rc)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if got["invoiceId"] != "inv-9" || got["amount"].(float64) != 42 || got["deep"] != true {
		t.Fatalf("resolved = %v", got)
	}
	if got["total"].(float64) != 7 {
		t.Fatalf("step ref = %v", got["total"])
	}
	if got["literal"] != "please keep $trigger.invoice_id inline" {
		t.Fatalf("inline ref must stay literal, got %v", got["literal"])
	}
	list := got["nested"].(map[string]any)["list"].([]any)
	if list[0] != "inv-9" || list[1].(float64) != 123 || list[2] != true {
		t.Fatalf("nested list = %v", list)
	}
	if got["missing"] != nil {
		t.Fatalf("missing ref = %v, want null", got["missing"])
	}
}

func TestResolveParamsEmptyAndInvalid(t *testing.T) {
	rc := testContext(t, `{}`, nil)
	out, err := resolveParams(nil, rc)
	if err != nil || string(out) != "{}" {
		t.Fatalf("empty params = %s, %v", out, err)
	}
	if _, err := resolveParams(json.RawMessage(`{bad`), rc); err == nil {
		t.Fatal("invalid JSON accepted")
	}
}
