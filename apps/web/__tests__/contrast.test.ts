import { readFileSync } from "node:fs";
import path from "node:path";
import { describe, expect, it } from "vitest";

/**
 * Parses the design tokens in globals.css and verifies that every
 * foreground/background semantic pair meets WCAG AA contrast (4.5:1)
 * in both the light and dark palettes.
 */

const css = readFileSync(
  path.join(__dirname, "..", "app", "globals.css"),
  "utf8",
);

function parseBlock(selector: string): Record<string, string> {
  const pattern =
    selector === ":root"
      ? /@theme\s*\{([\s\S]*?)\n\}/
      : /\.dark\s*\{([\s\S]*?)\n\}/;
  const match = css.match(pattern);
  if (!match) throw new Error(`token block not found: ${selector}`);
  const tokens: Record<string, string> = {};
  for (const line of match[1].split("\n")) {
    const m = line.match(/--(color-[\w-]+):\s*(#[0-9a-fA-F]{3,8})\s*;/);
    if (m) tokens[m[1]] = m[2];
  }
  return tokens;
}

function luminance(hex: string): number {
  const h = hex.replace("#", "");
  const rgb = [0, 2, 4].map((i) => parseInt(h.slice(i, i + 2), 16) / 255);
  const linear = rgb.map((c) =>
    c <= 0.03928 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4),
  );
  return 0.2126 * linear[0] + 0.7152 * linear[1] + 0.0722 * linear[2];
}

function contrast(fg: string, bg: string): number {
  const [l1, l2] = [luminance(fg), luminance(bg)].sort((a, b) => b - a);
  return (l1 + 0.05) / (l2 + 0.05);
}

const PAIRS: Array<[string, string]> = [
  ["color-foreground", "color-background"],
  ["color-card-foreground", "color-card"],
  ["color-muted-foreground", "color-background"],
  ["color-primary-foreground", "color-primary"],
  ["color-secondary-foreground", "color-secondary"],
  ["color-accent-foreground", "color-accent"],
  ["color-destructive-foreground", "color-destructive"],
  ["color-success-muted-foreground", "color-success-muted"],
  ["color-warning-muted-foreground", "color-warning-muted"],
  ["color-error-muted-foreground", "color-error-muted"],
  ["color-info-muted-foreground", "color-info-muted"],
];

describe.each([
  ["light", ":root"],
  ["dark", ".dark"],
] as const)("token contrast (%s palette)", (_label, selector) => {
  const tokens = parseBlock(selector);

  it.each(PAIRS)("%s on %s >= 4.5:1", (fgToken, bgToken) => {
    const fg = tokens[fgToken];
    const bg = tokens[bgToken];
    expect(fg, `${fgToken} defined`).toBeTruthy();
    expect(bg, `${bgToken} defined`).toBeTruthy();
    expect(contrast(fg, bg)).toBeGreaterThanOrEqual(4.5);
  });
});
