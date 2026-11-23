# Condition Expression Grammar

Condition nodes branch on a typed expression. The grammar is deliberately
limited — no function calls, no arithmetic, no arbitrary identifiers — so
expressions are always safe to store, validate, and evaluate.

## Grammar

```ebnf
expr       := orExpr
orExpr     := andExpr ( "or" andExpr )*
andExpr    := notExpr ( "and" notExpr )*
notExpr    := "not" notExpr | "(" expr ")" | comparison
comparison := operand ( cmp operand )?
operand    := ref | literal
ref        := "$trigger" "." path
            | "$steps" "." nodeId ".output" ( "." path )?
literal    := string | number | "true" | "false" | "null"
cmp        := "==" | "!=" | "<" | "<=" | ">" | ">="
path       := ident ( "." ident )*
```

- `nodeId` is the ID of a node in the same graph (e.g. `action-1`).
- Strings use single or double quotes with `\\` escapes.
- `and`/`or`/`not` are reserved words; anything else starting with a letter
  is an error, not an implicit variable.

## Typing rules

- Ordering operators (`<`, `<=`, `>`, `>=`) require numeric operands
  (references count as potentially-numeric and are checked again at runtime).
- Equality (`==`, `!=`) accepts any pair except a number literal compared
  directly to a string literal.
- A bare operand (no comparison) is only valid as a boolean term when it is
  a reference or a boolean literal: `$trigger.active`, `not $trigger.blocked`.

## Errors

Validation returns a human-readable message with a 1-based byte position,
e.g. `expression error at position 21: unknown identifier "bogus"`. The same
wording is produced by the Go validator (`internal/graph`) and the TypeScript
mirror (`apps/web/lib/editor/expr.ts`); both are exercised by tests so the
client and server always agree.

## Examples

```
$trigger.amount > 100 and $trigger.currency == "USD"
$steps.action-1.output.status == 200
not ($trigger.a < 1 or $trigger.b < 2)
$trigger.threshold <= -3.5
```
