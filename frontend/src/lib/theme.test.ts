import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

const css = readFileSync("src/app/globals.css", "utf8");
function block(selector: ":root" | ".dark") {
  return new RegExp(`${selector.replace(".", "\\.")}\\s*\\{([\\s\\S]*?)\\n\\}`).exec(css)?.[1] ?? "";
}
function token(name: string, selector: ":root" | ".dark" = ":root"): string {
  const value = new RegExp(`--${name}:\\s*([^;]+);`).exec(block(selector))?.[1]
    ?? (selector === ".dark" ? new RegExp(`--${name}:\\s*([^;]+);`).exec(block(":root"))?.[1] : undefined);
  if (!value) throw new Error(`Missing ${name}`);
  const alias = /^var\(--(.+)\)$/.exec(value);
  return alias ? token(alias[1], selector) : value;
}
function luminance(name: string, selector: ":root" | ".dark") {
  const hex = token(name, selector);
  if (!/^#[0-9a-f]{6}$/i.test(hex)) throw new Error(`Expected hex color for ${name}`);
  const [r, g, b] = [1, 3, 5].map((i) => {
    const value = parseInt(hex.slice(i, i + 2), 16) / 255;
    return value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4;
  });
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}
function contrast(a: string, b: string, selector: ":root" | ".dark") {
  const [hi, lo] = [luminance(a, selector), luminance(b, selector)].sort((x, y) => y - x);
  return (hi + 0.05) / (lo + 0.05);
}

describe("theme tokens", () => {
  it("keeps dark theme values and Emerald Green accent", () => {
    expect(token("background", ".dark")).toBe("#1c1c1c");
    expect(token("card", ".dark")).toBe("#202020");
    expect(token("primary", ".dark")).toBe("#3ecf8e");
    expect(token("primary", ":root")).toBe("#3ecf8e");
    expect(token("primary-foreground", ".dark")).toBe("#171717");
    expect(css).toContain("--radius-sm: 6px");
  });
  it.each([":root", ".dark"] as const)("keeps text and focus visible in %s", (selector) => {
    for (const surface of ["background", "card", "popover"]) {
      for (const text of ["foreground", "muted-foreground", "destructive", "pending"]) {
        expect(contrast(text, surface, selector)).toBeGreaterThanOrEqual(4.5);
      }
      expect(contrast("ring", surface, selector)).toBeGreaterThanOrEqual(3);
      expect(contrast("input", surface, selector)).toBeGreaterThanOrEqual(3);
    }
  });
  it.each([":root", ".dark"] as const)("keeps dark CTA text readable in %s", (selector) => {
    for (const fill of ["primary", "primary-deep"]) {
      expect(contrast("primary-foreground", fill, selector)).toBeGreaterThanOrEqual(4.5);
    }
  });
});
