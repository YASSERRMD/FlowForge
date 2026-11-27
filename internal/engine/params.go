package engine

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/YASSERRMD/FlowForge/internal/graph"
)

// resolveParams substitutes whole-string data references inside a node's
// configured params with values from the run context. Only exact references
// are resolved ("$trigger.a.b", "$steps.node.output.x"); strings that merely
// contain a dollar sign stay literal. A missing reference resolves to null so
// the action can decide whether it is required.
func resolveParams(raw json.RawMessage, rc *graph.Context) (json.RawMessage, error) {
	if len(raw) == 0 {
		return json.RawMessage(`{}`), nil
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, fmt.Errorf("params must be valid JSON: %w", err)
	}
	resolved, err := json.Marshal(resolveValue(value, rc))
	if err != nil {
		return nil, fmt.Errorf("marshal resolved params: %w", err)
	}
	return resolved, nil
}

func resolveValue(value any, rc *graph.Context) any {
	switch v := value.(type) {
	case string:
		if isDataReference(v) {
			if resolved, ok := graph.LookupReference(rc, v); ok {
				return resolved
			}
			return nil
		}
		return v
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, item := range v {
			out[key] = resolveValue(item, rc)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = resolveValue(item, rc)
		}
		return out
	default:
		return value
	}
}

func isDataReference(s string) bool {
	return strings.HasPrefix(s, "$trigger.") || strings.HasPrefix(s, "$steps.")
}
