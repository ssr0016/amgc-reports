import { describe, expect, it } from "vitest";
import { GENERATED_LENGTH, PASSWORD_CHARS, generatePassword } from "./password";

// A fixed random source: cycles through the bytes it is given.
const source = (...bytes: number[]) => {
  let i = 0;
  return (n: number) => Uint8Array.from({ length: n }, () => bytes[i++ % bytes.length]);
};
const all = PASSWORD_CHARS.lower + PASSWORD_CHARS.upper + PASSWORD_CHARS.digits;

describe("generatePassword", () => {
  it("is 20 characters from the alphabet, without the look-alikes 0 O 1 l I", () => {
    expect(GENERATED_LENGTH).toBe(20);
    for (let i = 0; i < 200; i++) {
      const pw = generatePassword();
      expect(pw).toHaveLength(20);
      for (const ch of pw) expect(all).toContain(ch);
      expect(pw).not.toMatch(/[0O1lI]/);
    }
  });

  it("always has a lower case letter, an upper case letter and a digit", () => {
    for (let i = 0; i < 200; i++) {
      const pw = generatePassword();
      expect(pw).toMatch(/[a-z]/);
      expect(pw).toMatch(/[A-Z]/);
      expect(pw).toMatch(/[2-9]/);
    }
  });

  it("is under bcrypt's 72 bytes, and two default calls differ", () => {
    const a = generatePassword();
    expect(new TextEncoder().encode(a).length).toBeLessThan(72);
    expect(generatePassword()).not.toBe(a);
  });

  it("is deterministic for a fixed random source, and follows it", () => {
    expect(generatePassword(source(1, 2, 3, 4, 5, 6, 7))).toBe(generatePassword(source(1, 2, 3, 4, 5, 6, 7)));
    expect(generatePassword(source(1, 2, 3, 4, 5, 6, 7))).not.toBe(generatePassword(source(9, 8, 7, 6, 5, 4, 3)));
  });

  it("skips random bytes that would bias the pick (rejection sampling)", () => {
    // 255 is above the largest multiple of every set size, so it must be thrown away, not wrapped.
    const pw = generatePassword(source(255, 255, 255, 0, 255, 255, 1, 255, 2));
    expect(pw).toHaveLength(20);
    for (const ch of pw) expect(all).toContain(ch);
  });

  it("shuffles: the guaranteed lower, upper and digit are not always first", () => {
    const firstClasses = new Set<string>();
    for (let i = 0; i < 200; i++) {
      const ch = generatePassword()[0];
      firstClasses.add(/[a-z]/.test(ch) ? "lower" : /[A-Z]/.test(ch) ? "upper" : "digit");
    }
    expect(firstClasses.size).toBeGreaterThan(1);
  });
});
