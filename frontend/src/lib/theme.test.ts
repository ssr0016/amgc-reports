import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

// Reads the real tokens in globals.css, so a theme edit that breaks contrast fails here.
const css = readFileSync("src/app/globals.css", "utf8") // vitest runs from the frontend directory;
const darkAt = css.indexOf("@media (prefers-color-scheme: dark) {");
const light = css.slice(css.indexOf(":root {"), darkAt);
const dark = css.slice(css.indexOf(":root {", darkAt));

// oklch -> linear sRGB (Ottosson), enough for a WCAG ratio.
function linear(block: string, token: string): number[] {
  const m = new RegExp(`--${token}:\\s*oklch\\(([\\d.]+)\\s+([\\d.]+)\\s+([\\d.]+)`).exec(block);
  if (!m) throw new Error(`--${token} not found`);
  const [L, C, h] = [Number(m[1]), Number(m[2]), (Number(m[3]) * Math.PI) / 180];
  const a = C * Math.cos(h);
  const b = C * Math.sin(h);
  const [l, mm, s] = [L + 0.3963377774 * a + 0.2158037573 * b, L - 0.1055613458 * a - 0.0638541728 * b, L - 0.0894841775 * a - 1.291485548 * b].map((x) => x ** 3);
  const rgb = [4.0767416621 * l - 3.3077115913 * mm + 0.2309699292 * s, -1.2684380046 * l + 2.6097574011 * mm - 0.3413193965 * s, -0.0041960863 * l - 0.7034186147 * mm + 1.7076147 * s];
  return rgb.map((x) => Math.min(1, Math.max(0, x)));
}
const luminance = ([r, g, b]: number[]) => 0.2126 * r + 0.7152 * g + 0.0722 * b;
function contrast(block: string, fg: string, bg: string) {
  const [hi, lo] = [luminance(linear(block, fg)), luminance(linear(block, bg))].sort((x, y) => y - x);
  return (hi + 0.05) / (lo + 0.05);
}

describe.each([
  ["light", light],
  ["dark", dark],
])("%s theme contrast", (_name, block) => {
  // WCAG 2.4.7 and 1.4.11: the focus indicator (the border-ring colour) needs 3:1.
  it("the focus ring against the background is at least 3:1", () => {
    expect(contrast(block, "ring", "background")).toBeGreaterThanOrEqual(3);
  });

  // WCAG 1.4.3: text.
  it.each(["muted-foreground", "destructive"])("%s text on the background is at least 4.5:1", (token) => {
    expect(contrast(block, token, "background")).toBeGreaterThanOrEqual(4.5);
  });
});
