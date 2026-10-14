import { useEffect, useState } from "react";
import { ApiError } from "./api";

// Only same-site paths: "/reports" is fine, "//evil.com" and "/\evil.com" are not.
export function safeNext(next: string | null): string {
  return next && /^\/(?![/\\])/.test(next) ? next : "/";
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
