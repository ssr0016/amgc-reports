import { act, renderHook } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ApiError } from "./api";
import { HOME, retryText, safeNext, toFormError, useCountdown } from "./auth-form";

describe("safeNext", () => {
  it("keeps same-site paths", () => {
    expect(safeNext("/reports?month=May")).toBe("/reports?month=May");
    expect(safeNext("/admin/users")).toBe("/admin/users");
  });

  it("uses the admin fallback for absent or unsafe destinations, while preserving safe paths", () => {
    expect(safeNext(null, "/admin")).toBe("/admin");
    expect(safeNext("//evil.com", "/admin")).toBe("/admin");
    expect(safeNext("/reports", "/admin")).toBe("/reports");
  });

  it.each([
    null,
    "",
    "reports",
    "//evil.com",
    "/\\evil.com",
    "/\t/evil.com",
    "/\n/evil.com",
    "/ok\u0000",
    "https://evil.com",
    "javascript:alert(1)",
  ])("sends %j home", (next) => {
    expect(safeNext(next)).toBe(HOME);
  });
});

describe("toFormError", () => {
  it("sends 422 details to their fields", () => {
    const err = new ApiError(422, "VALIDATION_ERROR", "bad", { email: "taken" });
    expect(toFormError(err)).toEqual({ message: "", fields: { email: "taken" } });
  });

  it("shows a message for other statuses, with an override per status", () => {
    const err = new ApiError(401, "UNAUTHORIZED", "invalid credentials");
    expect(toFormError(err)).toEqual({ message: "invalid credentials", fields: {} });
    expect(toFormError(err, { 401: "Wrong email or password." }).message).toBe("Wrong email or password.");
  });

  it("carries the wait on 429", () => {
    const err = new ApiError(429, "RATE_LIMIT", "slow", undefined, 30);
    expect(toFormError(err)).toEqual({ message: "Too many attempts.", fields: {}, retryAfter: 30 });
  });

  it("rethrows what is not an ApiError", () => {
    expect(() => toFormError(new Error("boom"))).toThrow("boom");
  });
});

describe("retryText", () => {
  it("appends the wait only when there is one", () => {
    expect(retryText(new ApiError(429, "R", "Slow down.", undefined, 5))).toBe("Slow down. Try again in 5s.");
    expect(retryText(new ApiError(500, "E", "Oops."))).toBe("Oops.");
  });
});

describe("useCountdown", () => {
  it("counts down one second at a time and stops at 0", () => {
    vi.useFakeTimers();
    const { result } = renderHook(() => useCountdown());
    expect(result.current[0]).toBe(0);
    act(() => result.current[1](2));
    expect(result.current[0]).toBe(2);
    act(() => void vi.advanceTimersByTime(1000));
    expect(result.current[0]).toBe(1);
    act(() => void vi.advanceTimersByTime(5000));
    expect(result.current[0]).toBe(0);
    vi.useRealTimers();
  });
});
