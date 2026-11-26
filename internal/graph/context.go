package graph

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// Context is the typed variable environment of one run: the trigger input
// plus the outputs of completed steps. Values are plain decoded JSON
// (objects, arrays, strings, numbers, booleans and null), never Go structs,
// so the same document round-trips deterministically.
//
// Version starts at 1 when the trigger is loaded and increments on every
// merge, so consumers can order context snapshots without comparing payload
// bytes.
type Context struct {
	Version int
	trigger any
	steps   map[string]any
}

// NewContext decodes the trigger input (a JSON object) into a context.
func NewContext(trigger json.RawMessage) (*Context, error) {
	c := &Context{Version: 1, steps: map[string]any{}}
	if len(trigger) == 0 {
		c.trigger = map[string]any{}
		return c, nil
	}
	var root any
	if err := json.Unmarshal(trigger, &root); err != nil {
		return nil, fmt.Errorf("decode trigger: %w", err)
	}
	if _, ok := root.(map[string]any); !ok {
		return nil, fmt.Errorf("trigger must be a JSON object")
	}
	c.trigger = root
	return c, nil
}

// MergeStep records the output of a completed node and bumps the version.
// Re-merging a node (a retried execution) replaces its previous output.
func (c *Context) MergeStep(nodeID string, output json.RawMessage) error {
	if nodeID == "" {
		return fmt.Errorf("empty node id")
	}
	var value any
	if len(output) == 0 {
		value = map[string]any{}
	} else if err := json.Unmarshal(output, &value); err != nil {
		return fmt.Errorf("decode output of %s: %w", nodeID, err)
	}
	if c.steps == nil {
		c.steps = map[string]any{}
	}
	c.steps[nodeID] = value
	c.Version++
	return nil
}

// TriggerValue resolves a dotted path under the trigger input.
func (c *Context) TriggerValue(path []string) (any, bool) {
	return LookupPath(c.trigger, path)
}

// StepValue resolves a dotted path under a node's output document.
func (c *Context) StepValue(nodeID string, path []string) (any, bool) {
	root, ok := c.steps[nodeID]
	if !ok {
		return nil, false
	}
	return LookupPath(root, path)
}

// HasStep reports whether a node output is present in the context.
func (c *Context) HasStep(nodeID string) bool {
	_, ok := c.steps[nodeID]
	return ok
}

// LookupPath walks nested JSON by object key or array index.
func LookupPath(root any, path []string) (any, bool) {
	current := root
	for _, segment := range path {
		switch node := current.(type) {
		case map[string]any:
			value, ok := node[segment]
			if !ok {
				return nil, false
			}
			current = value
		case []any:
			index, err := strconv.Atoi(segment)
			if err != nil || index < 0 || index >= len(node) {
				return nil, false
			}
			current = node[index]
		default:
			return nil, false
		}
	}
	return current, true
}
