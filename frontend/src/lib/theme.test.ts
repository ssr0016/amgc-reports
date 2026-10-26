import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

const css = readFileSync("src/app/globals.css", "utf8");
function token(name: string): string {
  const value = new RegExp(`--${name}:\\s*([^;]+);`).exec(css)?.[1];
  if (!value) throw new Error(`Missing ${name}`);
  const alias = /^var\(--(.+)\)$/.exec(value);
  return alias ? token(alias[1]) : value;
}
function luminance(name: string) {
  const hex = token(name);
  if (!/^#[0-9a-f]{6}$/i.test(hex)) throw new Error(`Expected hex color for ${name}`);
  const [r, g, b] = [1, 3, 5].map((i) => {
    const value = parseInt(hex.slice(i, i + 2), 16) / 255;
    return value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4;
  });
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}
function contrast(a: string, b: string) {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x);
  return (hi + 0.05) / (lo + 0.05);
}

describe("DESIGN.md dark theme", () => {
  it("uses the required canvas and primary colors", () => {
    expect(token("background")).toBe("#1c1c1c");
    expect(token("card")).toBe("#202020");
    expect(token("primary")).toBe("#3ecf8e");
    expect(token("primary-foreground")).toBe("#171717");
    expect(css).toContain("--radius-sm: 6px");
  });
  it.each(["background", "card", "popover"])("keeps text and focus visible on %s", (surface) => {
    for (const text of ["foreground", "muted-foreground", "destructive", "primary", "pending"]) {
      expect(contrast(text, surface)).toBeGreaterThanOrEqual(4.5);
    }
    expect(contrast("ring", surface)).toBeGreaterThanOrEqual(3);
    expect(contrast("input", surface)).toBeGreaterThanOrEqual(3);
  });
  it.each(["primary", "primary-deep"])("keeps dark CTA text readable on %s", (fill) => {
    expect(contrast("primary-foreground", fill)).toBeGreaterThanOrEqual(4.5);
  });
});
