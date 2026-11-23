package templates

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

// TestTemplatesMatchContractSchema cross-checks the JSON schema artifact
// against the Go validator constants so the two cannot drift apart.
func TestTemplatesMatchContractSchema(t *testing.T) {
	schema, err := os.ReadFile(filepath.Join("..", "..", "packages", "contracts", "graph-schema.json"))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(schema, &doc); err != nil {
		t.Fatal(err)
	}
	if got := doc["properties"].(map[string]any)["schemaVersion"].(map[string]any)["const"]; got != graph.SchemaVersion {
		t.Errorf("schema const %v != graph.SchemaVersion %q", got, graph.SchemaVersion)
	}
	if got := doc["properties"].(map[string]any)["nodes"].(map[string]any)["maxItems"]; int(got.(float64)) != graph.MaxNodes {
		t.Errorf("schema maxItems %v != graph.MaxNodes %d", got, graph.MaxNodes)
	}
	if got := doc["properties"].(map[string]any)["edges"].(map[string]any)["maxItems"]; int(got.(float64)) != graph.MaxEdges {
		t.Errorf("schema maxItems %v != graph.MaxEdges %d", got, graph.MaxEdges)
	}

	// Every template references only registered mock action types.
	for _, tmpl := range List() {
		parsed, err := tmpl.ParseGraph()
		if err != nil {
			t.Fatal(err)
		}
		for _, node := range parsed.Nodes {
			if node.Kind != graph.KindAction {
				continue
			}
			var cfg struct {
				ActionType string `json:"actionType"`
			}
			if err := json.Unmarshal(node.Config, &cfg); err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(cfg.ActionType, "mock.") {
				t.Errorf("template %q action %q uses non-mock action type %q", tmpl.ID, node.ID, cfg.ActionType)
			}
		}
	}
}
