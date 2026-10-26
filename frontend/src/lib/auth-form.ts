import { useEffect, useState } from "react";
import { ApiError } from "./api";
import { HOME } from "./routes";

export { HOME };

// Only same-site paths: "/reports" is fine, "//evil.com" and "/\evil.com" are not.
// Browsers drop tabs and newlines inside a URL, so "/\t/evil.com" would become
// "//evil.com"; control characters are rejected, and the resolved origin is checked.
export function safeNext(next: string | null, fallback: string = HOME): string {
  if (!next || !/^\/(?![/\\])/.test(next) || /[\u0000-\u001F\u007F]/.test(next)) return fallback;
  return new URL(next, location.origin).origin === location.origin ? next : fallback;
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

// An API error's message, with the wait when the server asked for one (429).
export function retryText(err: ApiError) {
  return err.message + (err.retryAfter ? ` Try again in ${err.retryAfter}s.` : "");
}

// What a failed request shows: the server's message (with the wait on a 429), else fallback.
export function errorText(err: unknown, fallback: string) {
  return err instanceof ApiError ? retryText(err) : fallback;
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
