package graph

import (
	"fmt"
	"strconv"
	"strings"
)

// Condition expression grammar (see docs/contracts/conditions.md):
//
//	expr       := orExpr
//	orExpr     := andExpr ( "or" andExpr )*
//	andExpr    := notExpr ( "and" notExpr )*
//	notExpr    := "not" notExpr | "(" expr ")" | comparison
//	comparison := operand ( cmp operand )?
//	operand    := ref | literal
//	ref        := "$trigger" path | "$steps" "." ident ".output" path
//	literal    := string | number | "true" | "false" | "null"
//	cmp        := "==" | "!=" | "<" | "<=" | ">" | ">="
//
// The grammar is deliberately non-Turing-complete: no function calls, no
// arithmetic, no arbitrary identifiers. Operands must be comparable pairs
// (number vs number, string vs string, bool vs bool).

type exprTokenKind int

const (
	tokRef exprTokenKind = iota
	tokString
	tokNumber
	tokBool
	tokNull
	tokCmp
	tokAnd
	tokOr
	tokNot
	tokLParen
	tokRParen
)

type exprToken struct {
	kind  exprTokenKind
	text  string
	pos   int // byte offset in the source
	value any // decoded literal value for string/number/bool
}

type exprError struct {
	msg string
	pos int
}

func (e *exprError) Error() string {
	return fmt.Sprintf("expression error at position %d: %s", e.pos+1, e.msg)
}

func lexExpr(src string) ([]exprToken, error) {
	var tokens []exprToken
	i := 0
	for i < len(src) {
		c := src[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		case c == '(':
			tokens = append(tokens, exprToken{kind: tokLParen, text: "(", pos: i})
			i++
		case c == ')':
			tokens = append(tokens, exprToken{kind: tokRParen, text: ")", pos: i})
			i++
		case c == '$':
			start := i
			for i < len(src) && (isIdentChar(src[i]) || src[i] == '$' || src[i] == '.' || src[i] == '-') {
				i++
			}
			text := src[start:i]
			if err := validateRef(text); err != "" {
				return nil, &exprError{msg: err, pos: start}
			}
			tokens = append(tokens, exprToken{kind: tokRef, text: text, pos: start})
		case c == '"' || c == '\'':
			start := i
			quote := c
			i++
			var sb strings.Builder
			for i < len(src) && src[i] != quote {
				if src[i] == '\\' && i+1 < len(src) {
					i++
					switch src[i] {
					case 'n':
						sb.WriteByte('\n')
					case 't':
						sb.WriteByte('\t')
					default:
						sb.WriteByte(src[i])
					}
					i++
					continue
				}
				sb.WriteByte(src[i])
				i++
			}
			if i >= len(src) {
				return nil, &exprError{msg: "unterminated string literal", pos: start}
			}
			i++
			tokens = append(tokens, exprToken{kind: tokString, text: src[start:i], pos: start, value: sb.String()})
		case c == '-' || (c >= '0' && c <= '9'):
			start := i
			if c == '-' {
				i++
			}
			for i < len(src) && (src[i] >= '0' && src[i] <= '9' || src[i] == '.') {
				i++
			}
			text := src[start:i]
			n, err := strconv.ParseFloat(text, 64)
			if err != nil {
				return nil, &exprError{msg: fmt.Sprintf("invalid number %q", text), pos: start}
			}
			tokens = append(tokens, exprToken{kind: tokNumber, text: text, pos: start, value: n})
		case c == '=' || c == '!' || c == '<' || c == '>':
			start := i
			if i+1 < len(src) && src[i+1] == '=' {
				i += 2
			} else if c == '<' || c == '>' {
				i++
			} else {
				return nil, &exprError{msg: fmt.Sprintf("expected %q after %q", "=", string(c)), pos: start}
			}
			tokens = append(tokens, exprToken{kind: tokCmp, text: src[start:i], pos: start})
		case isIdentStart(c):
			start := i
			for i < len(src) && isIdentChar(src[i]) {
				i++
			}
			word := src[start:i]
			switch word {
			case "and":
				tokens = append(tokens, exprToken{kind: tokAnd, text: word, pos: start})
			case "or":
				tokens = append(tokens, exprToken{kind: tokOr, text: word, pos: start})
			case "not":
				tokens = append(tokens, exprToken{kind: tokNot, text: word, pos: start})
			case "true":
				tokens = append(tokens, exprToken{kind: tokBool, text: word, pos: start, value: true})
			case "false":
				tokens = append(tokens, exprToken{kind: tokBool, text: word, pos: start, value: false})
			case "null":
				tokens = append(tokens, exprToken{kind: tokNull, text: word, pos: start})
			default:
				return nil, &exprError{
					msg: fmt.Sprintf("unknown identifier %q; use $trigger.… or $steps.… to reference data", word),
					pos: start,
				}
			}
		default:
			return nil, &exprError{msg: fmt.Sprintf("unexpected character %q", string(c)), pos: i}
		}
	}
	return tokens, nil
}

func isIdentStart(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_'
}

func isIdentChar(c byte) bool {
	return isIdentStart(c) || c >= '0' && c <= '9' || c == '_'
}

// validateRef checks a data reference against the two allowed roots.
func validateRef(ref string) string {
	if strings.HasPrefix(ref, "$trigger.") && len(ref) > len("$trigger.") {
		return validPath(ref[len("$trigger."):], ref)
	}
	const stepsPrefix = "$steps."
	if strings.HasPrefix(ref, stepsPrefix) {
		rest := ref[len(stepsPrefix):]
		dot := strings.Index(rest, ".output")
		if dot <= 0 {
			return fmt.Sprintf("malformed reference %q: expected $steps.<nodeId>.output.<field>", ref)
		}
		nodeID := rest[:dot]
		if !idPattern.MatchString(nodeID) {
			return fmt.Sprintf("malformed reference %q: %q is not a valid node ID", ref, nodeID)
		}
		rest = rest[dot+len(".output"):]
		if rest == "" {
			return "" // whole-output reference
		}
		if !strings.HasPrefix(rest, ".") || len(rest) == 1 {
			return fmt.Sprintf("malformed reference %q: expected .output.<field>", ref)
		}
		return validPath(rest[1:], ref)
	}
	return fmt.Sprintf("unknown reference root %q: use $trigger.<field> or $steps.<nodeId>.output.<field>", ref)
}

func validPath(path, ref string) string {
	for _, segment := range strings.Split(path, ".") {
		if segment == "" || !isIdentStart(segment[0]) {
			return fmt.Sprintf("malformed reference %q: invalid path segment %q", ref, segment)
		}
		for i := 0; i < len(segment); i++ {
			if !isIdentChar(segment[i]) {
				return fmt.Sprintf("malformed reference %q: invalid character in %q", ref, segment)
			}
		}
	}
	return ""
}

type exprValueKind int

const (
	valBool exprValueKind = iota
	valNumber
	valString
	valNull
	valRef
)

type exprParser struct {
	tokens []exprToken
	i      int
}

func (p *exprParser) peek() *exprToken {
	if p.i < len(p.tokens) {
		return &p.tokens[p.i]
	}
	return nil
}

func (p *exprParser) next() *exprToken {
	t := p.peek()
	if t != nil {
		p.i++
	}
	return t
}

func (p *exprParser) parseOr() error {
	if err := p.parseAnd(); err != nil {
		return err
	}
	for p.peek() != nil && p.peek().kind == tokOr {
		p.next()
		if err := p.parseAnd(); err != nil {
			return err
		}
	}
	return nil
}

func (p *exprParser) parseAnd() error {
	if err := p.parseNot(); err != nil {
		return err
	}
	for p.peek() != nil && p.peek().kind == tokAnd {
		p.next()
		if err := p.parseNot(); err != nil {
			return err
		}
	}
	return nil
}

func (p *exprParser) parseNot() error {
	t := p.peek()
	if t == nil {
		return &exprError{msg: "unexpected end of expression", pos: 0}
	}
	switch t.kind {
	case tokNot:
		p.next()
		return p.parseNot()
	case tokLParen:
		p.next()
		if err := p.parseOr(); err != nil {
			return err
		}
		closing := p.next()
		if closing == nil || closing.kind != tokRParen {
			return &exprError{msg: "missing closing parenthesis", pos: t.pos}
		}
		return nil
	default:
		return p.parseComparison()
	}
}

func operandKind(t *exprToken) exprValueKind {
	switch t.kind {
	case tokRef:
		return valRef
	case tokNumber:
		return valNumber
	case tokString:
		return valString
	case tokBool, tokNull:
		return valBool
	}
	return valRef
}

func (p *exprParser) parseComparison() error {
	left := p.next()
	if left == nil {
		return &exprError{msg: "unexpected end of expression", pos: 0}
	}
	if left.kind != tokRef && left.kind != tokNumber && left.kind != tokString && left.kind != tokBool && left.kind != tokNull {
		return &exprError{msg: fmt.Sprintf("expected a value or reference, found %q", left.text), pos: left.pos}
	}

	cmp := p.peek()
	if cmp == nil || cmp.kind != tokCmp {
		// Bare operand: only refs and bools are valid boolean terms.
		if left.kind == tokRef || left.kind == tokBool {
			return nil
		}
		return &exprError{msg: "expected a comparison operator (==, !=, <, <=, >, >=)", pos: left.pos}
	}
	p.next()

	right := p.next()
	if right == nil {
		return &exprError{msg: fmt.Sprintf("missing value after %q", cmp.text), pos: cmp.pos}
	}
	if right.kind != tokRef && right.kind != tokNumber && right.kind != tokString && right.kind != tokBool && right.kind != tokNull {
		return &exprError{msg: fmt.Sprintf("expected a value or reference after %q, found %q", cmp.text, right.text), pos: right.pos}
	}

	// Type compatibility: ordering operators need two numbers; equality
	// forbids mixing number and string literals.
	lt, rt := operandKind(left), operandKind(right)
	if cmp.text != "==" && cmp.text != "!=" {
		if (lt != valNumber && lt != valRef) || (rt != valNumber && rt != valRef) {
			return &exprError{msg: fmt.Sprintf("operator %q requires numeric operands", cmp.text), pos: cmp.pos}
		}
	}
	if (lt == valNumber && rt == valString) || (lt == valString && rt == valNumber) {
		return &exprError{msg: "cannot compare a number with a string", pos: cmp.pos}
	}
	return nil
}

// ValidateExpression parses a condition expression and returns "" when it is
// valid, otherwise a human-readable error with a 1-based position.
func ValidateExpression(src string) string {
	if strings.TrimSpace(src) == "" {
		return "expression must not be empty"
	}
	tokens, err := lexExpr(src)
	if err != nil {
		return err.Error()
	}
	p := &exprParser{tokens: tokens}
	if err := p.parseOr(); err != nil {
		return err.Error()
	}
	if t := p.peek(); t != nil {
		return (&exprError{msg: fmt.Sprintf("unexpected trailing %q", t.text), pos: t.pos}).Error()
	}
	return ""
}
