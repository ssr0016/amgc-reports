// A random password an admin can read out or paste: 20 characters, no look-alikes (0 O 1 l I).
// 20 characters from 57 symbols is far above the server's 60-bit rule and under bcrypt's 72 bytes.
export const GENERATED_LENGTH = 20;
export const PASSWORD_CHARS = {
  lower: "abcdefghijkmnopqrstuvwxyz",
  upper: "ABCDEFGHJKLMNPQRSTUVWXYZ",
  digits: "23456789",
} as const;

// n random bytes. Tests pass a fixed one; the default is the browser's CSPRNG.
type RandomBytes = (n: number) => Uint8Array;
const browserRandom: RandomBytes = (n) => crypto.getRandomValues(new Uint8Array(n));

// A uniform integer in [0, size). Bytes at or above the largest multiple of size are thrown
// away, so no value is more likely than another (a plain `byte % size` would be biased).
function below(size: number, random: RandomBytes): number {
  const limit = 256 - (256 % size);
  for (;;) {
    for (const byte of random(16)) if (byte < limit) return byte % size;
  }
}
const pick = (set: string, random: RandomBytes) => set[below(set.length, random)];

export function generatePassword(random: RandomBytes = browserRandom): string {
  const all = PASSWORD_CHARS.lower + PASSWORD_CHARS.upper + PASSWORD_CHARS.digits;
  // One of each class first, so the server's strength rule always has all three, then fill and shuffle.
  const chars = [pick(PASSWORD_CHARS.lower, random), pick(PASSWORD_CHARS.upper, random), pick(PASSWORD_CHARS.digits, random)];
  while (chars.length < GENERATED_LENGTH) chars.push(pick(all, random));
  for (let i = chars.length - 1; i > 0; i--) {
    const j = below(i + 1, random);
    [chars[i], chars[j]] = [chars[j], chars[i]];
  }
  return chars.join("");
}
