// Package templates ships the deterministic built-in starter workflows.
// Graphs are Go literals so seeding is reproducible byte-for-byte across
// environments; every template is validated against the graph contract by
// tests in this package.
package templates

import "encoding/json"

// SourceBuiltIn labels templates shipped with the platform (template source
// labeling for the gallery).
const SourceBuiltIn = "built-in"

// Template is a ready-to-run sample workflow.
type Template struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Tags        []string        `json:"tags"`
	Source      string          `json:"source"`
	Graph       json.RawMessage `json:"graph"`
}

var registry = []Template{
	invoiceApproval,
	parallelServiceRequest,
	incidentEscalation,
}

// List returns every built-in template in stable registry order.
func List() []Template {
	out := make([]Template, len(registry))
	copy(out, registry)
	return out
}

// Get returns one template by ID.
func Get(id string) (Template, bool) {
	for _, t := range registry {
		if t.ID == id {
			return t, true
		}
	}
	return Template{}, false
}
