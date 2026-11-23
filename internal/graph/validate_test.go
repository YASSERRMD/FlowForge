package graph

import (
	"encoding/json"
	"fmt"
	"testing"
)

func baseGraph() *Graph {
	cfg, _ := json.Marshal(map[string]string{"actionType": "http.request"})
	return &Graph{
		SchemaVersion: SchemaVersion,
		Nodes: []Node{
			{ID: "start-1", Kind: KindStart, Label: "Start"},
			{ID: "action-1", Kind: KindAction, Label: "Fetch", Config: cfg},
			{ID: "end-1", Kind: KindEnd, Label: "End"},
		},
		Edges: []Edge{
			{ID: "e-1", Source: "start-1", Target: "action-1"},
			{ID: "e-2", Source: "action-1", Target: "end-1"},
		},
	}
}

func codes(g *Graph) []string {
	var out []string
	for _, issue := range Validate(g) {
		out = append(out, issue.Code)
	}
	return out
}

func hasCode(list []string, want string) bool {
	return contains(list, want)
}

func TestValidateAcceptsMinimalGraph(t *testing.T) {
	if issues := Validate(baseGraph()); len(issues) != 0 {
		t.Fatalf("expected no issues, got %+v", issues)
	}
}

func TestValidateRejectsUnknownVersion(t *testing.T) {
	g := baseGraph()
	g.SchemaVersion = "2"
	if !hasCode(codes(g), "schema_version") {
		t.Fatal("expected schema_version issue")
	}
}

func TestValidateStartEndRules(t *testing.T) {
	g := baseGraph()
	g.Nodes = g.Nodes[1:]
	g.Edges = g.Edges[1:]
	if !hasCode(codes(g), "missing_start") {
		t.Fatal("expected missing_start")
	}

	g = baseGraph()
	g.Nodes = append(g.Nodes, Node{ID: "start-2", Kind: KindStart, Label: "S2"})
	if !hasCode(codes(g), "multiple_start") {
		t.Fatal("expected multiple_start")
	}

	g = baseGraph()
	g.Nodes = g.Nodes[:2]
	g.Edges = g.Edges[:1]
	if !hasCode(codes(g), "missing_end") {
		t.Fatal("expected missing_end")
	}
}

func TestValidateUnreachableAndOrphan(t *testing.T) {
	g := baseGraph()
	cfg, _ := json.Marshal(map[string]string{"actionType": "http.request"})
	g.Nodes = append(g.Nodes, Node{ID: "action-2", Kind: KindAction, Label: "Detached", Config: cfg})
	list := codes(g)
	if !hasCode(list, "unreachable_node") {
		t.Fatal("expected unreachable_node")
	}
	if !hasCode(list, "orphan_node") {
		t.Fatal("expected orphan_node")
	}
}

func TestValidateCycle(t *testing.T) {
	g := baseGraph()
	cfg, _ := json.Marshal(map[string]string{"actionType": "http.request"})
	g.Nodes = append(g.Nodes, Node{ID: "action-2", Kind: KindAction, Label: "B", Config: cfg})
	g.Edges = append(g.Edges,
		Edge{ID: "e-3", Source: "action-1", Target: "action-2"},
		Edge{ID: "e-4", Source: "action-2", Target: "action-1"},
	)
	if !hasCode(codes(g), "cycle") {
		t.Fatal("expected cycle")
	}
}

func TestValidateDuplicatesAndDangling(t *testing.T) {
	g := baseGraph()
	g.Nodes = append(g.Nodes, Node{ID: "action-1", Kind: KindAction, Label: "Dup"})
	g.Edges = append(g.Edges,
		Edge{ID: "e-1", Source: "action-1", Target: "end-1"},
		Edge{ID: "e-9", Source: "ghost", Target: "end-1"},
	)
	list := codes(g)
	for _, want := range []string{"duplicate_node_id", "duplicate_edge_id", "duplicate_edge", "dangling_edge"} {
		if !hasCode(list, want) {
			t.Fatalf("expected %s in %v", want, list)
		}
	}
}

func TestValidatePortRules(t *testing.T) {
	g := baseGraph()
	exprCfg, _ := json.Marshal(map[string]string{"expression": "$trigger.a > 1"})
	g.Nodes = append(g.Nodes, Node{ID: "condition-1", Kind: KindCondition, Label: "C", Config: exprCfg})
	maybe := "maybe"
	g.Edges = append(g.Edges,
		Edge{ID: "e-3", Source: "start-1", Target: "condition-1"},
		Edge{ID: "e-4", Source: "condition-1", SourceHandle: &maybe, Target: "end-1"},
	)
	if !hasCode(codes(g), "invalid_source_handle") {
		t.Fatal("expected invalid_source_handle for bad condition handle")
	}
	tru := "true"
	g.Edges[3].SourceHandle = &tru
	if hasCode(codes(g), "invalid_source_handle") {
		t.Fatal("did not expect invalid_source_handle for true handle")
	}

	g = baseGraph()
	g.Nodes = append(g.Nodes, Node{ID: "end-2", Kind: KindEnd, Label: "E2"})
	g.Edges = append(g.Edges,
		Edge{ID: "e-3", Source: "end-1", Target: "end-2"},
		Edge{ID: "e-4", Source: "action-1", Target: "start-1"},
	)
	list := codes(g)
	if !hasCode(list, "invalid_source_handle") || !hasCode(list, "invalid_target_kind") {
		t.Fatalf("expected endpoint kind violations, got %v", list)
	}
}

func TestValidateSizeLimits(t *testing.T) {
	g := baseGraph()
	for i := 0; i < MaxNodes; i++ {
		g.Nodes = append(g.Nodes, Node{ID: fmt.Sprintf("action-x%d", i), Kind: KindAction, Label: "A"})
	}
	if !hasCode(codes(g), "size_limit") {
		t.Fatal("expected size_limit")
	}
}

func TestValidateConfigRules(t *testing.T) {
	g := baseGraph()
	g.Nodes[1].Config = nil
	if !hasCode(codes(g), "config") {
		t.Fatal("expected config issue for missing actionType")
	}

	g = baseGraph()
	bad, _ := json.Marshal(map[string]int{"durationMs": 0})
	g.Nodes = append(g.Nodes, Node{ID: "delay-1", Kind: KindDelay, Label: "D", Config: bad})
	g.Edges = append(g.Edges,
		Edge{ID: "e-3", Source: "action-1", Target: "delay-1"},
		Edge{ID: "e-4", Source: "delay-1", Target: "end-1"},
	)
	if !hasCode(codes(g), "config") {
		t.Fatal("expected config issue for durationMs < 1")
	}
}

func TestCanonicalRoundTrip(t *testing.T) {
	g := baseGraph()
	first, err := g.MarshalCanonical()
	if err != nil {
		t.Fatal(err)
	}
	var parsed Graph
	if err := json.Unmarshal(first, &parsed); err != nil {
		t.Fatal(err)
	}
	second, err := parsed.MarshalCanonical()
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("round-trip not byte-stable:\n%s\n%s", first, second)
	}
}
