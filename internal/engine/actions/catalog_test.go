package actions

import (
	"context"
	"encoding/json"
	"testing"
)

func run(t *testing.T, reg *Registry, actionType, input string) map[string]any {
	t.Helper()
	out, err := reg.Execute(context.Background(), actionType, json.RawMessage(input))
	if err != nil {
		t.Fatalf("%s: %v", actionType, err)
	}
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("%s output: %v", actionType, err)
	}
	return m
}

func TestCatalogTypesRegistered(t *testing.T) {
	reg := Default()
	for _, typ := range CatalogTypes() {
		if !reg.Has(typ) {
			t.Fatalf("%s missing from Default registry", typ)
		}
	}
	if len(CatalogTypes()) < 15 {
		t.Fatalf("catalog too small: %d", len(CatalogTypes()))
	}
}

func TestInvoiceFetchIsDeterministic(t *testing.T) {
	reg := Default()
	a := run(t, reg, "mock.invoice.fetch", `{"invoiceId":"inv-123"}`)
	b := run(t, reg, "mock.invoice.fetch", `{"invoiceId":"inv-123"}`)
	if a["amount"] != b["amount"] || a["vendor"] != b["vendor"] {
		t.Fatalf("non-deterministic: %v vs %v", a, b)
	}
	if a["amount"].(float64) <= 0 || a["ok"] != true {
		t.Fatalf("unexpected fetch output: %v", a)
	}
	explicit := run(t, reg, "mock.invoice.fetch", `{"invoiceId":"x","amount":12.5,"currency":"EUR","vendor":"V"}`)
	if explicit["amount"].(float64) != 12.5 || explicit["currency"] != "EUR" || explicit["vendor"] != "V" {
		t.Fatalf("explicit fields ignored: %v", explicit)
	}
}

func TestInvoiceValidateAndDecisions(t *testing.T) {
	reg := Default()
	ok := run(t, reg, "mock.invoice.validate", `{"invoiceId":"inv-1","amount":50,"currency":"USD"}`)
	if ok["valid"] != true {
		t.Fatalf("valid invoice rejected: %v", ok)
	}
	bad := run(t, reg, "mock.invoice.validate", `{"invoiceId":"","amount":0,"currency":""}`)
	if bad["valid"] != false || len(bad["issues"].([]any)) != 3 {
		t.Fatalf("invalid invoice accepted: %v", bad)
	}

	approved := run(t, reg, "mock.invoice.approve", `{"invoiceId":"inv-1"}`)
	if approved["approved"] != true || approved["status"] != "approved" {
		t.Fatalf("approve = %v", approved)
	}
	rejected := run(t, reg, "mock.invoice.reject", `{"invoiceId":"inv-1"}`)
	if rejected["rejected"] != true || rejected["approved"] != false {
		t.Fatalf("reject = %v", rejected)
	}
	requested := run(t, reg, "mock.review.request", `{"assignee":"ada","invoiceId":"inv-1"}`)
	if requested["requested"] != true || requested["assignee"] != "ada" {
		t.Fatalf("review = %v", requested)
	}
}

func TestNotificationAndIncidentActions(t *testing.T) {
	reg := Default()
	email := run(t, reg, "mock.notify.email", `{"to":"ada@example.com","subject":"Hi"}`)
	if email["sent"] != true || email["channel"] != "email" {
		t.Fatalf("email = %v", email)
	}
	noRecipient := run(t, reg, "mock.notify.email", `{}`)
	if noRecipient["sent"] != false {
		t.Fatalf("email without recipient must not be sent: %v", noRecipient)
	}
	status := run(t, reg, "mock.notify.statuspage", `{"incidentId":"inc-1","state":"investigating"}`)
	if status["channel"] != "statuspage" || status["target"] != "inc-1" {
		t.Fatalf("statuspage = %v", status)
	}
	acked := run(t, reg, "mock.pager.page", `{"onCall":"ada","acknowledged":true}`)
	if acked["paged"] != true || acked["acknowledged"] != true {
		t.Fatalf("page = %v", acked)
	}
	esc := run(t, reg, "mock.pager.escalate", `{"manager":"grace","incidentId":"inc-1"}`)
	if esc["escalated"] != true || esc["manager"] != "grace" {
		t.Fatalf("escalate = %v", esc)
	}
	evt := run(t, reg, "mock.event.emit", `{"event":"degraded","incidentId":"inc-1"}`)
	if evt["emitted"] != true || evt["event"] != "degraded" {
		t.Fatalf("emit = %v", evt)
	}
}

func TestRequestAndCheckActions(t *testing.T) {
	reg := Default()
	if got := run(t, reg, "mock.request.load", `{"requestId":"r1"}`); got["loaded"] != true {
		t.Fatalf("load = %v", got)
	}
	if got := run(t, reg, "mock.request.fulfill", `{"requestId":"r1"}`); got["fulfilled"] != true {
		t.Fatalf("fulfill = %v", got)
	}
	if got := run(t, reg, "mock.request.reject", `{"requestId":"r1"}`); got["rejected"] != true {
		t.Fatalf("reject = %v", got)
	}
	for _, typ := range []string{"mock.check.inventory", "mock.check.credit", "mock.check.address"} {
		ok := run(t, reg, typ, `{"sku":"SKU-1","customerId":"c1","addressId":"a1"}`)
		if ok["ok"] != true {
			t.Fatalf("%s normal case = %v", typ, ok)
		}
		fail := run(t, reg, typ, `{"sku":"FAIL-1","customerId":"FAIL-1","addressId":"FAIL-1"}`)
		if fail["ok"] != false {
			t.Fatalf("%s forced failure = %v", typ, fail)
		}
	}
}

func TestCatalogRejectsMalformedInput(t *testing.T) {
	reg := Default()
	if _, err := reg.Execute(context.Background(), "mock.invoice.fetch", json.RawMessage(`[]`)); err == nil {
		t.Fatal("array input accepted")
	}
	if _, err := reg.Execute(context.Background(), "mock.invoice.fetch", json.RawMessage(`{bad`)); err == nil {
		t.Fatal("invalid JSON accepted")
	}
}
