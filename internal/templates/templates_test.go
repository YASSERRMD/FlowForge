package templates

import (
	"encoding/json"

	"testing"

	"github.com/YASSERRMD/FlowForge/internal/graph"
)

// TestTemplatesAreValidGraphs guards every registered template against the
// same server-side validator the publish path uses.
func TestTemplatesAreValidGraphs(t *testing.T) {
	for _, tmpl := range List() {
		t.Run(tmpl.ID, func(t *testing.T) {
			doc, err := tmpl.ParseGraph()
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			for _, issue := range graph.Validate(doc) {
				if issue.Severity == "error" {
					t.Errorf("template %q invalid: %+v", tmpl.ID, issue)
				}
			}
		})
	}
}

// TestTemplatesAreDeterministic guards seed reproducibility: registry order
// and serialized graphs are stable across calls.
func TestTemplatesAreDeterministic(t *testing.T) {
	first, second := List(), List()
	if len(first) != len(second) {
		t.Fatal("registry length changed between calls")
	}
	for i := range first {
		if first[i].ID != second[i].ID || string(first[i].Graph) != string(second[i].Graph) {
			t.Errorf("template %d not deterministic", i)
		}
	}
}

// ParseGraph decodes the template graph document.
func (t Template) ParseGraph() (*graph.Graph, error) {
	var doc graph.Graph
	if err := json.Unmarshal(t.Graph, &doc); err != nil {
		return nil, err
	}
	return &doc, nil
}
