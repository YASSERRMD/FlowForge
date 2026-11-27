// Package actions is the isolated registry of deterministic mock actions.
// Every registered action is pure: no wall-clock reads, no randomness, no
// I/O — the same input always produces the same output, which is what makes
// replayed executions byte-identical. Real external integrations are never
// called from here; the catalog grows in phase 09.
package actions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// ErrUnknownType is returned for action types absent from the registry.
var ErrUnknownType = errors.New("unknown action type")

// Action is one deterministic mock action with typed input and output.
type Action interface {
	// Type is the action identifier, e.g. "mock.echo".
	Type() string
	// Execute maps validated input to a deterministic output document.
	Execute(ctx context.Context, input json.RawMessage) (json.RawMessage, error)
}

// Registry holds the available actions.
type Registry struct {
	actions map[string]Action
}

// NewRegistry builds an empty registry.
func NewRegistry() *Registry { return &Registry{actions: map[string]Action{}} }

// Register adds an action; duplicate types panic (wiring error).
func (r *Registry) Register(a Action) {
	if _, exists := r.actions[a.Type()]; exists {
		panic("duplicate action registration: " + a.Type())
	}
	r.actions[a.Type()] = a
}

// Has reports whether the type is registered.
func (r *Registry) Has(actionType string) bool {
	_, ok := r.actions[actionType]
	return ok
}

// Types returns the registered action types in deterministic order.
func (r *Registry) Types() []string {
	types := make([]string, 0, len(r.actions))
	for t := range r.actions {
		types = append(types, t)
	}
	sortStrings(types)
	return types
}

// Execute runs the named action, mapping a missing type to ErrUnknownType.
func (r *Registry) Execute(ctx context.Context, actionType string, input json.RawMessage) (json.RawMessage, error) {
	a, ok := r.actions[actionType]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownType, actionType)
	}
	return a.Execute(ctx, input)
}

// Default builds the deterministic mock catalog: pass-through and failure
// actions plus the phase-09 business simulation catalog. Real external
// integrations are intentionally absent.
func Default() *Registry {
	r := NewRegistry()
	r.Register(Echo{})
	r.Register(Fail{})
	r.Register(InvoiceFetch{})
	r.Register(InvoiceValidate{})
	r.Register(InvoiceApprove{})
	r.Register(InvoiceReject{})
	r.Register(ReviewRequest{})
	r.Register(NotifyEmail{})
	r.Register(NotifyStatusPage{})
	r.Register(PagerPage{})
	r.Register(PagerEscalate{})
	r.Register(EventEmit{})
	r.Register(RequestLoad{})
	r.Register(RequestFulfill{})
	r.Register(RequestReject{})
	r.Register(CheckInventory{})
	r.Register(CheckCredit{})
	r.Register(CheckAddress{})
	return r
}

// Echo returns its input wrapped in a stable envelope. It exists so
// sequential graphs can pass data between steps before the phase-09
// catalog lands.
type Echo struct{}

// EchoOutput is the deterministic echo envelope.
type EchoOutput struct {
	OK   bool            `json:"ok"`
	Echo json.RawMessage `json:"echo"`
}

// Type implements Action.
func (Echo) Type() string { return "mock.echo" }

// Execute implements Action.
func (Echo) Execute(_ context.Context, input json.RawMessage) (json.RawMessage, error) {
	if len(input) == 0 {
		input = json.RawMessage(`{}`)
	}
	var obj map[string]any
	if err := json.Unmarshal(input, &obj); err != nil || obj == nil {
		return nil, fmt.Errorf("mock.echo input must be a JSON object")
	}
	out, err := json.Marshal(EchoOutput{OK: true, Echo: input})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Fail always fails with the configured message; it exists so tests and
// workflow authors can exercise durable failure paths.
type Fail struct{}

// FailInput configures the failure.
type FailInput struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Type implements Action.
func (Fail) Type() string { return "mock.fail" }

// Execute implements Action.
func (Fail) Execute(_ context.Context, input json.RawMessage) (json.RawMessage, error) {
	var in FailInput
	if len(input) > 0 {
		if err := json.Unmarshal(input, &in); err != nil {
			return nil, fmt.Errorf("mock.fail input must be a JSON object")
		}
	}
	if in.Message == "" {
		in.Message = "mock.fail requested"
	}
	return nil, errors.New(in.Message)
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
