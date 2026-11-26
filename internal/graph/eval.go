package graph

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"
)

// ValueScope resolves expression references at evaluation time. *Context
// implements it; tests and callers may supply any deterministic source.
type ValueScope interface {
	TriggerValue(path []string) (any, bool)
	StepValue(nodeID string, path []string) (any, bool)
}

// EvalError is a deterministic expression evaluation failure. It never wraps
// Go runtime panics or user code: evaluation is a pure function of the
// expression and the supplied scope.
type EvalError struct {
	Message string
}

func (e *EvalError) Error() string { return e.Message }

func evalErrorf(format string, args ...any) error {
	return &EvalError{Message: fmt.Sprintf(format, args...)}
}

// EvaluateExpression parses and evaluates a condition expression against the
// scope, returning its boolean result. Unsafe or malformed expressions return
// a validation/evaluation error instead of executing anything.
func EvaluateExpression(src string, scope ValueScope) (bool, error) {
	if strings.TrimSpace(src) == "" {
		return false, &EvalError{Message: "expression must not be empty"}
	}
	tokens, err := lexExpr(src)
	if err != nil {
		return false, err
	}
	p := &exprParser{tokens: tokens}
	root, err := p.parseOrNode()
	if err != nil {
		return false, err
	}
	if t := p.peek(); t != nil {
		return false, evalErrorf("unexpected trailing %q", t.text)
	}
	value, err := root.eval(scope)
	if err != nil {
		return false, err
	}
	result, ok := value.(bool)
	if !ok {
		return false, evalErrorf("expression result must be a boolean")
	}
	return result, nil
}

// exprNode is one node of the parsed expression tree.
type exprNode interface {
	eval(scope ValueScope) (any, error)
}

type literalNode struct{ value any }

func (n literalNode) eval(ValueScope) (any, error) { return n.value, nil }

type refNode struct{ ref string }

func (n refNode) eval(scope ValueScope) (any, error) {
	value, ok := resolveRef(scope, n.ref)
	if !ok {
		// A missing reference resolves to null so `== null` and `!= null`
		// remain meaningful and deterministic.
		return nil, nil
	}
	return value, nil
}

type compareNode struct {
	op    string
	left  exprNode
	right exprNode
}

func (n compareNode) eval(scope ValueScope) (any, error) {
	left, err := n.left.eval(scope)
	if err != nil {
		return nil, err
	}
	right, err := n.right.eval(scope)
	if err != nil {
		return nil, err
	}
	switch n.op {
	case "==":
		return valuesEqual(left, right), nil
	case "!=":
		return !valuesEqual(left, right), nil
	case "<", "<=", ">", ">=":
		lf, lok := asNumber(left)
		rf, rok := asNumber(right)
		if !lok || !rok {
			return nil, evalErrorf("operator %q requires numeric operands", n.op)
		}
		switch n.op {
		case "<":
			return lf < rf, nil
		case "<=":
			return lf <= rf, nil
		case ">":
			return lf > rf, nil
		default:
			return lf >= rf, nil
		}
	}
	return nil, evalErrorf("unknown operator %q", n.op)
}

type boolNode struct {
	op    string // "and", "or"
	left  exprNode
	right exprNode
}

func (n boolNode) eval(scope ValueScope) (any, error) {
	left, err := evalBool(n.left, scope)
	if err != nil {
		return nil, err
	}
	if n.op == "and" && !left {
		return false, nil
	}
	if n.op == "or" && left {
		return true, nil
	}
	return evalBool(n.right, scope)
}

type notNode struct{ inner exprNode }

func (n notNode) eval(scope ValueScope) (any, error) {
	value, err := evalBool(n.inner, scope)
	if err != nil {
		return nil, err
	}
	return !value, nil
}

func evalBool(n exprNode, scope ValueScope) (bool, error) {
	value, err := n.eval(scope)
	if err != nil {
		return false, err
	}
	if b, ok := value.(bool); ok {
		return b, nil
	}
	return false, evalErrorf("boolean term must evaluate to a boolean")
}

// resolveRef resolves "$trigger.<path>" and "$steps.<node>.output[.<path>]".
func resolveRef(scope ValueScope, ref string) (any, bool) {
	if scope == nil {
		return nil, false
	}
	const triggerPrefix = "$trigger."
	if strings.HasPrefix(ref, triggerPrefix) {
		return scope.TriggerValue(splitPath(ref[len(triggerPrefix):]))
	}
	const stepsPrefix = "$steps."
	rest := strings.TrimPrefix(ref, stepsPrefix)
	dot := strings.Index(rest, ".output")
	if dot <= 0 {
		return nil, false
	}
	nodeID := rest[:dot]
	rem := rest[dot+len(".output"):]
	if rem == "" {
		return scope.StepValue(nodeID, nil)
	}
	if !strings.HasPrefix(rem, ".") {
		return nil, false
	}
	return scope.StepValue(nodeID, splitPath(rem[1:]))
}

func splitPath(p string) []string {
	if p == "" {
		return nil
	}
	return strings.Split(p, ".")
}

// valuesEqual compares two decoded JSON values with numeric coercion so JSON
// numbers (always float64 after decoding) and Go integer values compare
// sanely.
func valuesEqual(a, b any) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	if an, ok := asNumber(a); ok {
		if bn, ok := asNumber(b); ok {
			return an == bn
		}
		return false
	}
	if ab, ok := a.(bool); ok {
		bb, ok := b.(bool)
		return ok && ab == bb
	}
	return reflect.DeepEqual(a, b)
}

func asNumber(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, !math.IsNaN(n)
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case int32:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

// parser additions: parseOrNode builds an evaluable tree while reusing the
// token stream produced by lexExpr. It mirrors ValidateExpression exactly.
func (p *exprParser) parseOrNode() (exprNode, error) {
	left, err := p.parseAndNode()
	if err != nil {
		return nil, err
	}
	for p.peek() != nil && p.peek().kind == tokOr {
		p.next()
		right, err := p.parseAndNode()
		if err != nil {
			return nil, err
		}
		left = boolNode{op: "or", left: left, right: right}
	}
	return left, nil
}

func (p *exprParser) parseAndNode() (exprNode, error) {
	left, err := p.parseNotNode()
	if err != nil {
		return nil, err
	}
	for p.peek() != nil && p.peek().kind == tokAnd {
		p.next()
		right, err := p.parseNotNode()
		if err != nil {
			return nil, err
		}
		left = boolNode{op: "and", left: left, right: right}
	}
	return left, nil
}

func (p *exprParser) parseNotNode() (exprNode, error) {
	t := p.peek()
	if t == nil {
		return nil, &exprError{msg: "unexpected end of expression", pos: 0}
	}
	switch t.kind {
	case tokNot:
		p.next()
		inner, err := p.parseNotNode()
		if err != nil {
			return nil, err
		}
		return notNode{inner: inner}, nil
	case tokLParen:
		p.next()
		inner, err := p.parseOrNode()
		if err != nil {
			return nil, err
		}
		closing := p.next()
		if closing == nil || closing.kind != tokRParen {
			return nil, &exprError{msg: "missing closing parenthesis", pos: t.pos}
		}
		return inner, nil
	default:
		return p.parseComparisonNode()
	}
}

func (p *exprParser) parseComparisonNode() (exprNode, error) {
	leftToken := p.next()
	if leftToken == nil {
		return nil, &exprError{msg: "unexpected end of expression", pos: 0}
	}
	if !isValueToken(leftToken) {
		return nil, &exprError{msg: fmt.Sprintf("expected a value or reference, found %q", leftToken.text), pos: leftToken.pos}
	}
	left := tokenNode(leftToken)

	cmp := p.peek()
	if cmp == nil || cmp.kind != tokCmp {
		if leftToken.kind == tokRef || leftToken.kind == tokBool {
			return left, nil
		}
		return nil, &exprError{msg: "expected a comparison operator (==, !=, <, <=, >, >=)", pos: leftToken.pos}
	}
	p.next()

	rightToken := p.next()
	if rightToken == nil {
		return nil, &exprError{msg: fmt.Sprintf("missing value after %q", cmp.text), pos: cmp.pos}
	}
	if !isValueToken(rightToken) {
		return nil, &exprError{msg: fmt.Sprintf("expected a value or reference after %q, found %q", cmp.text, rightToken.text), pos: rightToken.pos}
	}

	lt, rt := operandKind(leftToken), operandKind(rightToken)
	if cmp.text != "==" && cmp.text != "!=" {
		if (lt != valNumber && lt != valRef) || (rt != valNumber && rt != valRef) {
			return nil, &exprError{msg: fmt.Sprintf("operator %q requires numeric operands", cmp.text), pos: cmp.pos}
		}
	}
	if (lt == valNumber && rt == valString) || (lt == valString && rt == valNumber) {
		return nil, &exprError{msg: "cannot compare a number with a string", pos: cmp.pos}
	}
	return compareNode{op: cmp.text, left: left, right: tokenNode(rightToken)}, nil
}

func isValueToken(t *exprToken) bool {
	switch t.kind {
	case tokRef, tokNumber, tokString, tokBool, tokNull:
		return true
	}
	return false
}

func tokenNode(t *exprToken) exprNode {
	switch t.kind {
	case tokRef:
		return refNode{ref: t.text}
	case tokNull:
		return literalNode{value: nil}
	default:
		return literalNode{value: t.value}
	}
}

// LookupReference resolves a serialized reference (e.g. "$trigger.a.b") for
// callers that build node inputs from configured parameters. It returns
// ok=false for malformed references or missing values.
func LookupReference(scope ValueScope, ref string) (any, bool) {
	return resolveRef(scope, ref)
}
