// Package graph defines the canonical workflow DAG contract (schemaVersion
// "1") and its server-side validation. Keep the rules in sync with the
// TypeScript mirror in apps/web/lib/editor/validate.ts and the JSON schema
// in packages/contracts/graph-schema.json.
package graph

import (
	"encoding/json"
	"fmt"
)

// SchemaVersion is the only graph document version this build accepts.
const SchemaVersion = "1"

const (
	MaxNodes = 200
	MaxEdges = 400
)

// NodeKind enumerates the seven supported node types.
type NodeKind string

const (
	KindStart     NodeKind = "start"
	KindEnd       NodeKind = "end"
	KindAction    NodeKind = "action"
	KindDelay     NodeKind = "delay"
	KindCondition NodeKind = "condition"
	KindFork      NodeKind = "fork"
	KindJoin      NodeKind = "join"
)

// Position is the editor canvas coordinate of a node.
type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Node is one vertex of the workflow DAG. Config is per-kind and open:
// unknown keys are preserved verbatim on round-trip.
type Node struct {
	ID       string          `json:"id"`
	Kind     NodeKind        `json:"kind"`
	Label    string          `json:"label"`
	Position *Position       `json:"position,omitempty"`
	Config   json.RawMessage `json:"config,omitempty"`
}

// Edge connects a source handle on one node to a target node.
type Edge struct {
	ID           string  `json:"id"`
	Source       string  `json:"source"`
	SourceHandle *string `json:"sourceHandle,omitempty"`
	Target       string  `json:"target"`
	TargetHandle *string `json:"targetHandle,omitempty"`
	Label        string  `json:"label,omitempty"`
}

// Graph is the canonical persisted workflow document.
type Graph struct {
	SchemaVersion string `json:"schemaVersion"`
	Nodes         []Node `json:"nodes"`
	Edges         []Edge `json:"edges"`
}

// MarshalCanonical serializes the graph with stable field order so an
// unedited load→save round-trip is byte-stable.
func (g *Graph) MarshalCanonical() ([]byte, error) {
	data, err := json.Marshal(g)
	if err != nil {
		return nil, fmt.Errorf("marshal graph: %w", err)
	}
	return data, nil
}

// SourceHandles returns the outgoing port names a kind may use, given its
// config (fork derives branch-N handles from branchCount).
func SourceHandles(kind NodeKind, config json.RawMessage) []string {
	switch kind {
	case KindStart, KindAction, KindDelay, KindJoin:
		return []string{"default"}
	case KindCondition:
		return []string{"true", "false"}
	case KindFork:
		n := branchCount(config)
		handles := make([]string, 0, n)
		for i := 1; i <= n; i++ {
			handles = append(handles, fmt.Sprintf("branch-%d", i))
		}
		return handles
	default:
		return nil
	}
}

func branchCount(config json.RawMessage) int {
	if len(config) == 0 {
		return 2
	}
	var parsed struct {
		BranchCount int `json:"branchCount"`
	}
	if err := json.Unmarshal(config, &parsed); err != nil || parsed.BranchCount < 2 {
		return 2
	}
	return parsed.BranchCount
}

// AcceptsIncoming reports whether the kind may have incoming edges.
func AcceptsIncoming(kind NodeKind) bool { return kind != KindStart }

// HasOutgoing reports whether the kind may originate edges.
func HasOutgoing(kind NodeKind) bool { return kind != KindEnd }

// ValidKind reports whether s is a known node kind.
func ValidKind(s string) bool {
	switch NodeKind(s) {
	case KindStart, KindEnd, KindAction, KindDelay, KindCondition, KindFork, KindJoin:
		return true
	}
	return false
}
