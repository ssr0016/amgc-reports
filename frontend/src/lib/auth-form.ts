import { useEffect, useState } from "react";
import { ApiError } from "./api";

// Where a signed-in user lands when there is no (safe) ?next=.
export const HOME = "/reports";

// Only same-site paths: "/reports" is fine, "//evil.com" and "/\evil.com" are not.
// Browsers drop tabs and newlines inside a URL, so "/\t/evil.com" would become
// "//evil.com"; control characters are rejected, and the resolved origin is checked.
export function safeNext(next: string | null): string {
  if (!next || !/^\/(?![/\\])/.test(next) || /[\u0000-\u001F\u007F]/.test(next)) return HOME;
  return new URL(next, location.origin).origin === location.origin ? next : HOME;
}

export type FormError = { message: string; fields: Record<string, string>; retryAfter?: number };

// Maps an API error onto the form: 422 details go to their fields, the rest to
// one message. `messages` overrides the text for a status.
export function toFormError(err: unknown, messages: Partial<Record<number, string>> = {}): FormError {
  if (!(err instanceof ApiError)) throw err;
  if (err.isValidation && err.details) return { message: "", fields: err.details };
  if (err.isRateLimited) {
    return { message: messages[429] ?? "Too many attempts.", fields: {}, retryAfter: err.retryAfter };
  }
  return { message: messages[err.status] ?? err.message, fields: {} };
}

// Counts down from the seconds it is given; 0 means idle.
export function useCountdown() {
  const [left, setLeft] = useState(0);
  useEffect(() => {
    if (left <= 0) return;
    const timer = setTimeout(() => setLeft(left - 1), 1000);
    return () => clearTimeout(timer);
  }, [left]);
  return [left, setLeft] as const;
}
