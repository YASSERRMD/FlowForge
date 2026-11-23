/**
 * TypeScript mirror of the condition grammar validator in
 * internal/graph/expr.go. Keep error wording in sync — see
 * docs/contracts/conditions.md.
 */

type TokenKind =
  | "ref"
  | "string"
  | "number"
  | "bool"
  | "null"
  | "cmp"
  | "and"
  | "or"
  | "not"
  | "lparen"
  | "rparen";

interface Token {
  kind: TokenKind;
  text: string;
  pos: number;
}

const ID_PATTERN = /^[a-zA-Z0-9_-]+$/;

function isIdentStart(c: string): boolean {
  return /[a-zA-Z_]/.test(c);
}

function isIdentChar(c: string): boolean {
  return /[a-zA-Z0-9_]/.test(c);
}

class ExprError extends Error {
  constructor(msg: string, pos: number) {
    super(`expression error at position ${pos + 1}: ${msg}`);
  }
}

function validPath(path: string, ref: string): string {
  for (const segment of path.split(".")) {
    if (segment === "" || !isIdentStart(segment[0])) {
      return `malformed reference "${ref}": invalid path segment "${segment}"`;
    }
    for (const c of segment) {
      if (!isIdentChar(c)) {
        return `malformed reference "${ref}": invalid character in "${segment}"`;
      }
    }
  }
  return "";
}

function validateRef(ref: string): string {
  if (ref.startsWith("$trigger.") && ref.length > "$trigger.".length) {
    return validPath(ref.slice("$trigger.".length), ref);
  }
  const stepsPrefix = "$steps.";
  if (ref.startsWith(stepsPrefix)) {
    const rest = ref.slice(stepsPrefix.length);
    const dot = rest.indexOf(".output");
    if (dot <= 0) {
      return `malformed reference "${ref}": expected $steps.<nodeId>.output.<field>`;
    }
    const nodeId = rest.slice(0, dot);
    if (!ID_PATTERN.test(nodeId)) {
      return `malformed reference "${ref}": "${nodeId}" is not a valid node ID`;
    }
    const tail = rest.slice(dot + ".output".length);
    if (tail === "") return "";
    if (!tail.startsWith(".") || tail.length === 1) {
      return `malformed reference "${ref}": expected .output.<field>`;
    }
    return validPath(tail.slice(1), ref);
  }
  return `unknown reference root "${ref}": use $trigger.<field> or $steps.<nodeId>.output.<field>`;
}

function lex(src: string): Token[] {
  const tokens: Token[] = [];
  let i = 0;
  while (i < src.length) {
    const c = src[i];
    if (c === " " || c === "\t" || c === "\n" || c === "\r") {
      i++;
    } else if (c === "(") {
      tokens.push({ kind: "lparen", text: "(", pos: i });
      i++;
    } else if (c === ")") {
      tokens.push({ kind: "rparen", text: ")", pos: i });
      i++;
    } else if (c === "$") {
      const start = i;
      while (
        i < src.length &&
        (isIdentChar(src[i]) || src[i] === "$" || src[i] === "." || src[i] === "-")
      ) {
        i++;
      }
      const text = src.slice(start, i);
      const err = validateRef(text);
      if (err) throw new ExprError(err, start);
      tokens.push({ kind: "ref", text, pos: start });
    } else if (c === '"' || c === "'") {
      const start = i;
      const quote = c;
      i++;
      while (i < src.length && src[i] !== quote) {
        i += src[i] === "\\" && i + 1 < src.length ? 2 : 1;
      }
      if (i >= src.length) throw new ExprError("unterminated string literal", start);
      i++;
      tokens.push({ kind: "string", text: src.slice(start, i), pos: start });
    } else if (c === "-" || /[0-9]/.test(c)) {
      const start = i;
      if (c === "-") i++;
      while (i < src.length && /[0-9.]/.test(src[i])) i++;
      const text = src.slice(start, i);
      if (Number.isNaN(Number(text))) {
        throw new ExprError(`invalid number "${text}"`, start);
      }
      tokens.push({ kind: "number", text, pos: start });
    } else if (c === "=" || c === "!" || c === "<" || c === ">") {
      const start = i;
      if (i + 1 < src.length && src[i + 1] === "=") {
        i += 2;
      } else if (c === "<" || c === ">") {
        i++;
      } else {
        throw new ExprError(`expected "=" after "${c}"`, start);
      }
      tokens.push({ kind: "cmp", text: src.slice(start, i), pos: start });
    } else if (isIdentStart(c)) {
      const start = i;
      while (i < src.length && isIdentChar(src[i])) i++;
      const word = src.slice(start, i);
      if (word === "and" || word === "or" || word === "not") {
        tokens.push({ kind: word, text: word, pos: start });
      } else if (word === "true" || word === "false") {
        tokens.push({ kind: "bool", text: word, pos: start });
      } else if (word === "null") {
        tokens.push({ kind: "null", text: word, pos: start });
      } else {
        throw new ExprError(
          `unknown identifier "${word}"; use $trigger.… or $steps.… to reference data`,
          start,
        );
      }
    } else {
      throw new ExprError(`unexpected character "${c}"`, i);
    }
  }
  return tokens;
}

type ValueKind = "bool" | "number" | "string" | "ref";

class Parser {
  private i = 0;
  constructor(private tokens: Token[]) {}

  private peek(): Token | undefined {
    return this.tokens[this.i];
  }

  private next(): Token | undefined {
    return this.tokens[this.i++];
  }

  parseOr(): void {
    this.parseAnd();
    while (this.peek()?.kind === "or") {
      this.next();
      this.parseAnd();
    }
  }

  ensureEnd(): void {
    const t = this.peek();
    if (t) throw new ExprError(`unexpected trailing "${t.text}"`, t.pos);
  }

  private parseAnd(): void {
    this.parseNot();
    while (this.peek()?.kind === "and") {
      this.next();
      this.parseNot();
    }
  }

  private parseNot(): void {
    const t = this.peek();
    if (!t) throw new ExprError("unexpected end of expression", 0);
    if (t.kind === "not") {
      this.next();
      this.parseNot();
    } else if (t.kind === "lparen") {
      this.next();
      this.parseOr();
      const closing = this.next();
      if (!closing || closing.kind !== "rparen") {
        throw new ExprError("missing closing parenthesis", t.pos);
      }
    } else {
      this.parseComparison();
    }
  }

  private parseComparison(): void {
    const left = this.next();
    if (!left) throw new ExprError("unexpected end of expression", 0);
    if (!["ref", "number", "string", "bool", "null"].includes(left.kind)) {
      throw new ExprError(`expected a value or reference, found "${left.text}"`, left.pos);
    }

    const cmp = this.peek();
    if (!cmp || cmp.kind !== "cmp") {
      if (left.kind === "ref" || left.kind === "bool") return;
      throw new ExprError(
        "expected a comparison operator (==, !=, <, <=, >, >=)",
        left.pos,
      );
    }
    this.next();

    const right = this.next();
    if (!right) throw new ExprError(`missing value after "${cmp.text}"`, cmp.pos);
    if (!["ref", "number", "string", "bool", "null"].includes(right.kind)) {
      throw new ExprError(
        `expected a value or reference after "${cmp.text}", found "${right.text}"`,
        right.pos,
      );
    }

    const lt = left.kind as ValueKind;
    const rt = right.kind as ValueKind;
    if (cmp.text !== "==" && cmp.text !== "!=") {
      if (
        (lt !== "number" && lt !== "ref") ||
        (rt !== "number" && rt !== "ref")
      ) {
        throw new ExprError(`operator "${cmp.text}" requires numeric operands`, cmp.pos);
      }
    }
    if (
      (lt === "number" && rt === "string") ||
      (lt === "string" && rt === "number")
    ) {
      throw new ExprError("cannot compare a number with a string", cmp.pos);
    }
  }
}

/**
 * Returns "" when the expression is valid, otherwise a human-readable
 * error with a 1-based byte position.
 */
export function validateExpression(src: string): string {
  if (src.trim() === "") return "expression must not be empty";
  try {
    const parser = new Parser(lex(src));
    parser.parseOr();
    parser.ensureEnd();
    return "";
  } catch (err) {
    if (err instanceof ExprError) return err.message;
    throw err;
  }
}
