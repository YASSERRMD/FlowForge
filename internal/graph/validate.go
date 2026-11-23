package graph

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var idPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// Issue is one structural or configuration problem found in a graph.
// Codes mirror the client validator (apps/web/lib/editor/validate.ts).
type Issue struct {
	Severity string `json:"severity"` // "error" | "warning"
	Code     string `json:"code"`
	Message  string `json:"message"`
	NodeID   string `json:"nodeId,omitempty"`
	EdgeID   string `json:"edgeId,omitempty"`
}

func errorIssue(code, message string) Issue {
	return Issue{Severity: "error", Code: code, Message: message}
}

// Validate checks a parsed graph against every structural invariant of the
// contract: size limits, ID uniqueness, start/end rules, port compatibility,
// reachability, acyclicity and per-node config requirements.
func Validate(g *Graph) []Issue {
	var issues []Issue

	if g.SchemaVersion != SchemaVersion {
		issues = append(issues, errorIssue("schema_version",
			fmt.Sprintf("unsupported schemaVersion %q; only %q is supported", g.SchemaVersion, SchemaVersion)))
	}

	if len(g.Nodes) > MaxNodes {
		issues = append(issues, errorIssue("size_limit",
			fmt.Sprintf("graph has %d nodes; the limit is %d", len(g.Nodes), MaxNodes)))
	}
	if len(g.Edges) > MaxEdges {
		issues = append(issues, errorIssue("size_limit",
			fmt.Sprintf("graph has %d edges; the limit is %d", len(g.Edges), MaxEdges)))
	}

	nodeByID := make(map[string]*Node, len(g.Nodes))
	for i := range g.Nodes {
		node := &g.Nodes[i]
		if node.ID == "" || len(node.ID) > 64 || !idPattern.MatchString(node.ID) {
			issues = append(issues, errorIssue("duplicate_node_id",
				fmt.Sprintf("node ID %q is empty, too long or contains invalid characters", node.ID)))
		}
		if !ValidKind(string(node.Kind)) {
			issues = append(issues, errorIssue("config",
				fmt.Sprintf("node %q has unknown kind %q", node.ID, node.Kind)))
		}
		if strings.TrimSpace(node.Label) == "" || len(node.Label) > 120 {
			issues = append(issues, errorIssue("config",
				fmt.Sprintf("node %q label must be 1-120 characters", node.ID)))
		}
		if _, dup := nodeByID[node.ID]; dup {
			issues = append(issues, errorIssue("duplicate_node_id",
				fmt.Sprintf("duplicate node ID %q", node.ID)))
			continue
		}
		nodeByID[node.ID] = node
	}

	var starts, ends int
	for i := range g.Nodes {
		switch g.Nodes[i].Kind {
		case KindStart:
			starts++
		case KindEnd:
			ends++
		}
	}
	if starts == 0 {
		issues = append(issues, errorIssue("missing_start", "graph must contain exactly one start node"))
	} else if starts > 1 {
		issues = append(issues, errorIssue("multiple_start", "graph must contain exactly one start node"))
	}
	if ends == 0 {
		issues = append(issues, errorIssue("missing_end", "graph must contain at least one end node"))
	}

	edgeIDs := make(map[string]struct{}, len(g.Edges))
	edgeKeys := make(map[string]struct{}, len(g.Edges))
	adjacency := make(map[string][]string, len(g.Nodes))
	inDegree := make(map[string]int, len(g.Nodes))

	for _, edge := range g.Edges {
		if _, dup := edgeIDs[edge.ID]; dup {
			issues = append(issues, errorIssue("duplicate_edge_id",
				fmt.Sprintf("duplicate edge ID %q", edge.ID)))
		}
		edgeIDs[edge.ID] = struct{}{}

		if edge.Source == edge.Target {
			issues = append(issues, Issue{Severity: "error", Code: "self_loop", EdgeID: edge.ID,
				Message: fmt.Sprintf("edge %q connects node %q to itself", edge.ID, edge.Source)})
		}

		source, sourceOK := nodeByID[edge.Source]
		target, targetOK := nodeByID[edge.Target]
		if !sourceOK || !targetOK {
			missing := edge.Source
			if sourceOK {
				missing = edge.Target
			}
			issues = append(issues, Issue{Severity: "error", Code: "dangling_edge", EdgeID: edge.ID,
				Message: fmt.Sprintf("edge %q references a missing node (%s)", edge.ID, missing)})
			continue
		}

		sh := ""
		if edge.SourceHandle != nil {
			sh = *edge.SourceHandle
		}
		key := edge.Source + "" + sh + "→" + edge.Target
		if _, dup := edgeKeys[key]; dup {
			issues = append(issues, Issue{Severity: "error", Code: "duplicate_edge", EdgeID: edge.ID,
				Message: fmt.Sprintf("duplicate edge from %q to %q", edge.Source, edge.Target)})
		}
		edgeKeys[key] = struct{}{}

		if !HasOutgoing(source.Kind) {
			issues = append(issues, Issue{Severity: "error", Code: "invalid_source_handle", EdgeID: edge.ID, NodeID: source.ID,
				Message: fmt.Sprintf("node %q (%s) cannot have outgoing edges", source.ID, source.Kind)})
		} else {
			handle := sh
			if handle == "" {
				handle = "default"
			}
			if !contains(SourceHandles(source.Kind, source.Config), handle) {
				issues = append(issues, Issue{Severity: "error", Code: "invalid_source_handle", EdgeID: edge.ID, NodeID: source.ID,
					Message: fmt.Sprintf("edge %q uses unknown source handle %q on node %q", edge.ID, handle, source.ID)})
			}
		}

		if !AcceptsIncoming(target.Kind) {
			issues = append(issues, Issue{Severity: "error", Code: "invalid_target_kind", EdgeID: edge.ID, NodeID: target.ID,
				Message: fmt.Sprintf("node %q (%s) cannot have incoming edges", target.ID, target.Kind)})
		}

		adjacency[edge.Source] = append(adjacency[edge.Source], edge.Target)
		inDegree[edge.Target]++
	}

	issues = append(issues, detectCycle(g, adjacency)...)

	if starts == 1 {
		reachable := map[string]bool{}
		var startID string
		for i := range g.Nodes {
			if g.Nodes[i].Kind == KindStart {
				startID = g.Nodes[i].ID
			}
		}
		stack := []string{startID}
		for len(stack) > 0 {
			id := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if reachable[id] {
				continue
			}
			reachable[id] = true
			stack = append(stack, adjacency[id]...)
		}
		for i := range g.Nodes {
			if !reachable[g.Nodes[i].ID] {
				issues = append(issues, Issue{Severity: "error", Code: "unreachable_node", NodeID: g.Nodes[i].ID,
					Message: fmt.Sprintf("node %q is not reachable from the start node", g.Nodes[i].ID)})
			}
		}
	}

	if len(g.Nodes) > 2 {
		for i := range g.Nodes {
			node := &g.Nodes[i]
			if node.Kind == KindStart || node.Kind == KindEnd {
				continue
			}
			if inDegree[node.ID] == 0 && len(adjacency[node.ID]) == 0 {
				issues = append(issues, Issue{Severity: "warning", Code: "orphan_node", NodeID: node.ID,
					Message: fmt.Sprintf("node %q (%s) is not connected to anything", node.ID, node.Label)})
			}
		}
	}

	for i := range g.Nodes {
		issues = append(issues, validateNodeConfig(&g.Nodes[i])...)
	}

	return issues
}

func detectCycle(g *Graph, adjacency map[string][]string) []Issue {
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int, len(g.Nodes))
	var issues []Issue
	var visit func(id string)
	visit = func(id string) {
		if len(issues) > 0 {
			return
		}
		color[id] = gray
		for _, next := range adjacency[id] {
			if color[next] == gray {
				issues = append(issues, Issue{Severity: "error", Code: "cycle", NodeID: next,
					Message: fmt.Sprintf("graph contains a cycle involving node %q", next)})
				return
			}
			if color[next] == white {
				visit(next)
			}
		}
		color[id] = black
	}
	for i := range g.Nodes {
		if color[g.Nodes[i].ID] == white {
			visit(g.Nodes[i].ID)
		}
	}
	return issues
}

// validateNodeConfig enforces required per-kind config keys server-side.
// Unknown keys are allowed (forward compatibility).
func validateNodeConfig(node *Node) []Issue {
	cfg := map[string]json.RawMessage{}
	if len(node.Config) > 0 {
		if err := json.Unmarshal(node.Config, &cfg); err != nil {
			return []Issue{{Severity: "error", Code: "config", NodeID: node.ID,
				Message: fmt.Sprintf("%s: config must be a JSON object", node.Label)}}
		}
	}

	requireNumber := func(key string, min float64) *Issue {
		raw, ok := cfg[key]
		if !ok {
			return &Issue{Severity: "error", Code: "config", NodeID: node.ID,
				Message: fmt.Sprintf("%s: %s is required", node.Label, key)}
		}
		var n float64
		if err := json.Unmarshal(raw, &n); err != nil || n < min || n != float64(int64(n)) {
			return &Issue{Severity: "error", Code: "config", NodeID: node.ID,
				Message: fmt.Sprintf("%s: %s must be an integer >= %v", node.Label, key, min)}
		}
		return nil
	}

	var issue *Issue
	switch node.Kind {
	case KindAction:
		raw, ok := cfg["actionType"]
		var s string
		if !ok || json.Unmarshal(raw, &s) != nil || s == "" {
			issue = &Issue{Severity: "error", Code: "config", NodeID: node.ID,
				Message: fmt.Sprintf("%s: actionType is required", node.Label)}
		}
	case KindDelay:
		issue = requireNumber("durationMs", 1)
	case KindCondition:
		raw, ok := cfg["expression"]
		var s string
		if !ok || json.Unmarshal(raw, &s) != nil || strings.TrimSpace(s) == "" {
			return []Issue{{Severity: "error", Code: "config", NodeID: node.ID,
				Message: fmt.Sprintf("%s: expression is required", node.Label)}}
		}
		if msg := ValidateExpression(s); msg != "" {
			return []Issue{{Severity: "error", Code: "config", NodeID: node.ID,
				Message: fmt.Sprintf("%s: %s", node.Label, msg)}}
		}
	case KindFork, KindJoin:
		issue = requireNumber("branchCount", 2)
	}
	if issue != nil {
		return []Issue{*issue}
	}
	return nil
}

func contains(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}
