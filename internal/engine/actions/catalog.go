// Deterministic mock action catalog. Every action here is a pure function of
// its input document: no clocks, no randomness, no network and no filesystem.
// That keeps replayed executions byte-identical and lets the platform
// demonstrate real branching, approvals and notifications without paid or
// external services.
package actions

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"strings"
)

// CatalogTypes lists the deterministic catalog added in phase 09. It is
// exported so tests and API validation can assert the exact surface.
func CatalogTypes() []string {
	r := Default()
	return r.Types()
}

func decodeInput(input json.RawMessage, out any) error {
	if len(input) == 0 {
		input = json.RawMessage(`{}`)
	}
	if err := json.Unmarshal(input, out); err != nil {
		return fmt.Errorf("action input must be a JSON object: %w", err)
	}
	return nil
}

func encodeOutput(out any) (json.RawMessage, error) {
	data, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("encode action output: %w", err)
	}
	return data, nil
}

// stableInt returns a deterministic non-negative integer derived from s.
func stableInt(s string) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return int(h.Sum32())
}

// okUnlessFails lets fixtures force a failed check by prefixing the identifier
// with "FAIL", while every normal identifier passes deterministically.
func okUnlessFails(id string) bool {
	return id != "" && !strings.HasPrefix(strings.ToUpper(id), "FAIL")
}

// InvoiceFetch simulates loading an invoice. When no amount is supplied it is
// derived deterministically from the invoice ID.
type InvoiceFetch struct{}

func (InvoiceFetch) Type() string { return "mock.invoice.fetch" }

type invoiceFetchInput struct {
	InvoiceID string   `json:"invoiceId"`
	Amount    *float64 `json:"amount"`
	Currency  string   `json:"currency"`
	Vendor    string   `json:"vendor"`
}

type invoiceFetchOutput struct {
	InvoiceID string  `json:"invoiceId"`
	Amount    float64 `json:"amount"`
	Currency  string  `json:"currency"`
	Vendor    string  `json:"vendor"`
	OK        bool    `json:"ok"`
}

func (InvoiceFetch) Execute(_ context.Context, input json.RawMessage) (json.RawMessage, error) {
	var in invoiceFetchInput
	if err := decodeInput(input, &in); err != nil {
		return nil, err
	}
	amount := 100 + float64(stableInt(in.InvoiceID)%900)
	if in.Amount != nil {
		amount = *in.Amount
	}
	currency := in.Currency
	if currency == "" {
		currency = "USD"
	}
	vendor := in.Vendor
	if vendor == "" {
		vendor = "Acme Supplies"
	}
	return encodeOutput(invoiceFetchOutput{
		InvoiceID: in.InvoiceID, Amount: amount, Currency: currency, Vendor: vendor, OK: true,
	})
}

// InvoiceValidate performs a deterministic synthetic validation.
type InvoiceValidate struct{}

func (InvoiceValidate) Type() string { return "mock.invoice.validate" }

type invoiceValidateInput struct {
	InvoiceID string   `json:"invoiceId"`
	Amount    *float64 `json:"amount"`
	Currency  string   `json:"currency"`
}

type invoiceValidateOutput struct {
	InvoiceID string   `json:"invoiceId"`
	Amount    float64  `json:"amount"`
	Valid     bool     `json:"valid"`
	Issues    []string `json:"issues"`
}

func (InvoiceValidate) Execute(_ context.Context, input json.RawMessage) (json.RawMessage, error) {
	var in invoiceValidateInput
	if err := decodeInput(input, &in); err != nil {
		return nil, err
	}
	amount := 0.0
	if in.Amount != nil {
		amount = *in.Amount
	}
	var issues []string
	if in.InvoiceID == "" {
		issues = append(issues, "invoiceId is required")
	}
	if amount <= 0 {
		issues = append(issues, "amount must be positive")
	}
	if in.Currency == "" {
		issues = append(issues, "currency is required")
	}
	return encodeOutput(invoiceValidateOutput{
		InvoiceID: in.InvoiceID, Amount: amount, Valid: len(issues) == 0, Issues: issues,
	})
}

// InvoiceApprove simulates a business approval.
type InvoiceApprove struct{}

func (InvoiceApprove) Type() string { return "mock.invoice.approve" }

type invoiceApproveInput struct {
	InvoiceID string `json:"invoiceId"`
}

type invoiceApproveOutput struct {
	InvoiceID string `json:"invoiceId"`
	Approved  bool   `json:"approved"`
	Status    string `json:"status"`
}

func (InvoiceApprove) Execute(_ context.Context, input json.RawMessage) (json.RawMessage, error) {
	var in invoiceApproveInput
	if err := decodeInput(input, &in); err != nil {
		return nil, err
	}
	return encodeOutput(invoiceApproveOutput{InvoiceID: in.InvoiceID, Approved: true, Status: "approved"})
}

// InvoiceReject simulates a business rejection with an optional reason.
type InvoiceReject struct{}

func (InvoiceReject) Type() string { return "mock.invoice.reject" }

type invoiceRejectInput struct {
	InvoiceID string `json:"invoiceId"`
	Reason    string `json:"reason"`
}

type invoiceRejectOutput struct {
	InvoiceID string `json:"invoiceId"`
	Approved  bool   `json:"approved"`
	Rejected  bool   `json:"rejected"`
	Reason    string `json:"reason"`
}

func (InvoiceReject) Execute(_ context.Context, input json.RawMessage) (json.RawMessage, error) {
	var in invoiceRejectInput
	if err := decodeInput(input, &in); err != nil {
		return nil, err
	}
	reason := in.Reason
	if reason == "" {
		reason = "rejected by reviewer"
	}
	return encodeOutput(invoiceRejectOutput{
		InvoiceID: in.InvoiceID, Approved: false, Rejected: true, Reason: reason,
	})
}

// ReviewRequest simulates routing a document to a human reviewer.
type ReviewRequest struct{}

func (ReviewRequest) Type() string { return "mock.review.request" }

type reviewRequestInput struct {
	Assignee  string `json:"assignee"`
	InvoiceID string `json:"invoiceId"`
}

func (ReviewRequest) Execute(_ context.Context, input json.RawMessage) (json.RawMessage, error) {
	var in reviewRequestInput
	if err := decodeInput(input, &in); err != nil {
		return nil, err
	}
	return encodeOutput(map[string]any{
		"requested": true, "assignee": in.Assignee, "invoiceId": in.InvoiceID,
	})
}

// notifyResult is shared by the notification actions. The executor persists
// an auditable in-app notification for these outputs; nothing is sent.
type notifyResult struct {
	Sent    bool   `json:"sent"`
	Channel string `json:"channel"`
	To      string `json:"to,omitempty"`
	Subject string `json:"subject,omitempty"`
	Target  string `json:"target,omitempty"`
	State   string `json:"state,omitempty"`
}

// NotifyEmail records an in-app email notification.
type NotifyEmail struct{}

func (NotifyEmail) Type() string { return "mock.notify.email" }

func (NotifyEmail) Execute(_ context.Context, input json.RawMessage) (json.RawMessage, error) {
	var in struct {
		To      string `json:"to"`
		Subject string `json:"subject"`
		Body    string `json:"body"`
	}
	if err := decodeInput(input, &in); err != nil {
		return nil, err
	}
	return encodeOutput(notifyResult{
		Sent: in.To != "", Channel: "email", To: in.To, Subject: in.Subject,
	})
}

// NotifyStatusPage records an in-app status-page notification.
type NotifyStatusPage struct{}

func (NotifyStatusPage) Type() string { return "mock.notify.statuspage" }

func (NotifyStatusPage) Execute(_ context.Context, input json.RawMessage) (json.RawMessage, error) {
	var in struct {
		IncidentID string `json:"incidentId"`
		State      string `json:"state"`
	}
	if err := decodeInput(input, &in); err != nil {
		return nil, err
	}
	return encodeOutput(notifyResult{
		Sent: true, Channel: "statuspage", Target: in.IncidentID, State: in.State,
	})
}

// PagerPage simulates paging an on-call engineer.
type PagerPage struct{}

func (PagerPage) Type() string { return "mock.pager.page" }

type pagerPageInput struct {
	OnCall       string `json:"onCall"`
	IncidentID   string `json:"incidentId"`
	Acknowledged bool   `json:"acknowledged"`
}

func (PagerPage) Execute(_ context.Context, input json.RawMessage) (json.RawMessage, error) {
	var in pagerPageInput
	if err := decodeInput(input, &in); err != nil {
		return nil, err
	}
	return encodeOutput(map[string]any{
		"paged": true, "acknowledged": in.Acknowledged,
		"onCall": in.OnCall, "incidentId": in.IncidentID,
	})
}

// PagerEscalate simulates escalating an incident.
type PagerEscalate struct{}

func (PagerEscalate) Type() string { return "mock.pager.escalate" }

func (PagerEscalate) Execute(_ context.Context, input json.RawMessage) (json.RawMessage, error) {
	var in struct {
		Manager    string `json:"manager"`
		IncidentID string `json:"incidentId"`
	}
	if err := decodeInput(input, &in); err != nil {
		return nil, err
	}
	return encodeOutput(map[string]any{
		"escalated": true, "manager": in.Manager, "incidentId": in.IncidentID,
	})
}

// EventEmit records a synthetic timeline event in the step output.
type EventEmit struct{}

func (EventEmit) Type() string { return "mock.event.emit" }

func (EventEmit) Execute(_ context.Context, input json.RawMessage) (json.RawMessage, error) {
	var in struct {
		Event      string `json:"event"`
		IncidentID string `json:"incidentId"`
	}
	if err := decodeInput(input, &in); err != nil {
		return nil, err
	}
	return encodeOutput(map[string]any{
		"emitted": true, "event": in.Event, "incidentId": in.IncidentID,
	})
}

// RequestLoad simulates loading a service request.
type RequestLoad struct{}

func (RequestLoad) Type() string { return "mock.request.load" }

func (RequestLoad) Execute(_ context.Context, input json.RawMessage) (json.RawMessage, error) {
	var in struct {
		RequestID string `json:"requestId"`
	}
	if err := decodeInput(input, &in); err != nil {
		return nil, err
	}
	return encodeOutput(map[string]any{"loaded": true, "requestId": in.RequestID})
}

// RequestFulfill simulates fulfilling a request.
type RequestFulfill struct{}

func (RequestFulfill) Type() string { return "mock.request.fulfill" }

func (RequestFulfill) Execute(_ context.Context, input json.RawMessage) (json.RawMessage, error) {
	var in struct {
		RequestID string `json:"requestId"`
	}
	if err := decodeInput(input, &in); err != nil {
		return nil, err
	}
	return encodeOutput(map[string]any{"fulfilled": true, "requestId": in.RequestID})
}

// RequestReject simulates rejecting a request.
type RequestReject struct{}

func (RequestReject) Type() string { return "mock.request.reject" }

func (RequestReject) Execute(_ context.Context, input json.RawMessage) (json.RawMessage, error) {
	var in struct {
		RequestID string `json:"requestId"`
	}
	if err := decodeInput(input, &in); err != nil {
		return nil, err
	}
	return encodeOutput(map[string]any{"rejected": true, "requestId": in.RequestID})
}

// checkResult is the shared output of the synthetic validation checks.
type checkResult struct {
	Check string `json:"check"`
	OK    bool   `json:"ok"`
	Ref   string `json:"ref,omitempty"`
}

func runCheck(check, value string) (json.RawMessage, error) {
	return encodeOutput(checkResult{Check: check, OK: okUnlessFails(value), Ref: value})
}

// CheckInventory simulates an inventory check.
type CheckInventory struct{}

func (CheckInventory) Type() string { return "mock.check.inventory" }

func (CheckInventory) Execute(_ context.Context, input json.RawMessage) (json.RawMessage, error) {
	var in struct {
		SKU string `json:"sku"`
	}
	if err := decodeInput(input, &in); err != nil {
		return nil, err
	}
	return runCheck("inventory", in.SKU)
}

// CheckCredit simulates a credit check.
type CheckCredit struct{}

func (CheckCredit) Type() string { return "mock.check.credit" }

func (CheckCredit) Execute(_ context.Context, input json.RawMessage) (json.RawMessage, error) {
	var in struct {
		CustomerID string `json:"customerId"`
	}
	if err := decodeInput(input, &in); err != nil {
		return nil, err
	}
	return runCheck("credit", in.CustomerID)
}

// CheckAddress simulates an address check.
type CheckAddress struct{}

func (CheckAddress) Type() string { return "mock.check.address" }

func (CheckAddress) Execute(_ context.Context, input json.RawMessage) (json.RawMessage, error) {
	var in struct {
		AddressID string `json:"addressId"`
	}
	if err := decodeInput(input, &in); err != nil {
		return nil, err
	}
	return runCheck("address", in.AddressID)
}
