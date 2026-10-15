import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { api, ApiError, resetCsrf, setOnUnauthorized } from "./api";

const json = (status: number, body: unknown, headers: Record<string, string> = {}) =>
  new Response(JSON.stringify(body), { status, headers });

// Answers by path, in order, and records every call.
function stubFetch(...replies: Array<(path: string) => Response | Promise<Response>>) {
  const fetchMock = vi.fn<(url: string, init?: RequestInit) => Promise<Response>>(async (url) => {
    const next = replies.shift();
    if (!next) throw new Error(`unexpected fetch ${url}`);
    return next(url);
  });
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

// The ApiError a call rejects with; fails the test if it resolves.
const failure = (p: Promise<unknown>) =>
  p.then(
    () => {
      throw new Error("expected the call to reject");
    },
    (e: ApiError) => e,
  );

const pathOf = (call: unknown[]) => String(call[0]).replace(/^undefined/, "");
const headersOf = (call: unknown[]) => (call[1] as RequestInit).headers as Record<string, string>;

beforeEach(() => {
  resetCsrf();
  setOnUnauthorized(null);
});
afterEach(() => vi.unstubAllGlobals());

describe("api", () => {
  it("sends credentials and no CSRF token on GET", async () => {
    const f = stubFetch(() => json(200, { ok: 1 }));
    expect(await api("/reports")).toEqual({ ok: 1 });
    expect(f).toHaveBeenCalledTimes(1);
    expect((f.mock.calls[0][1] as RequestInit).credentials).toBe("include");
    expect(headersOf(f.mock.calls[0])["X-CSRF-Token"]).toBeUndefined();
  });

  it("fetches a CSRF token once and reuses it for later writes", async () => {
    const f = stubFetch(
      () => json(200, { csrf_token: "t1" }),
      () => json(201, { id: 1 }),
      () => json(201, { id: 2 }),
    );
    await api("/reports", { method: "POST", body: { month: "May" } });
    await api("/reports", { method: "POST", body: { month: "June" } });
    expect(f.mock.calls.map(pathOf)).toEqual(["/auth/csrf", "/reports", "/reports"]);
    expect(headersOf(f.mock.calls[1])).toMatchObject({ "X-CSRF-Token": "t1", "Content-Type": "application/json" });
    expect(f.mock.calls[1][1]).toMatchObject({ body: '{"month":"May"}' });
  });

  it("refetches the token and retries once on a CSRF 403", async () => {
    const f = stubFetch(
      () => json(200, { csrf_token: "old" }),
      () => json(403, { error: "FORBIDDEN", message: "invalid CSRF token" }),
      () => json(200, { csrf_token: "new" }),
      () => json(200, { done: true }),
    );
    expect(await api("/reports/1", { method: "PUT", body: {} })).toEqual({ done: true });
    expect(headersOf(f.mock.calls[3])["X-CSRF-Token"]).toBe("new");
  });

  it("does not retry a 403 that is not about CSRF", async () => {
    const f = stubFetch(
      () => json(200, { csrf_token: "t" }),
      () => json(403, { error: "FORBIDDEN", message: "missing permission" }),
    );
    await expect(api("/reports/1/complete", { method: "PATCH" })).rejects.toMatchObject({ status: 403 });
    expect(f).toHaveBeenCalledTimes(2);
  });

  it("returns undefined for 204", async () => {
    stubFetch(
      () => json(200, { csrf_token: "t" }),
      () => new Response(null, { status: 204 }),
    );
    expect(await api("/reports/1", { method: "DELETE" })).toBeUndefined();
  });

  it("maps an error body, 422 details and Retry-After onto ApiError", async () => {
    stubFetch(() => json(422, { error: "VALIDATION_ERROR", message: "bad", details: { month: "nope" } }));
    const err = await failure(api("/reports"));
    expect(err).toBeInstanceOf(ApiError);
    expect(err).toMatchObject({ status: 422, code: "VALIDATION_ERROR", message: "bad", details: { month: "nope" } });
    expect(err.isValidation).toBe(true);

    stubFetch(() => json(429, { error: "RATE_LIMIT", message: "slow" }, { "Retry-After": "7" }));
    const limited = await failure(api("/reports"));
    expect(limited.isRateLimited).toBe(true);
    expect(limited.retryAfter).toBe(7);

    stubFetch(() => json(409, { error: "CONFLICT", message: "exists" }));
    expect((await failure(api("/reports"))).isConflict).toBe(true);
  });

  it("falls back to a generic message when the error body is not JSON", async () => {
    stubFetch(() => new Response("<html>", { status: 502 }));
    const err = await failure(api("/reports"));
    expect(err).toMatchObject({ status: 502, code: "ERROR", message: "Something went wrong. Please try again." });
    expect(err.retryAfter).toBeUndefined();
  });

  it("turns a failed fetch into a network error (status 0)", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new TypeError("Failed to fetch")));
    const err = await failure(api("/reports"));
    expect(err.isNetwork).toBe(true);
    expect(err.message).toMatch(/Cannot reach the server/);
  });

  describe("401 handling", () => {
    it("calls the unauthorized hook", async () => {
      const hook = vi.fn();
      setOnUnauthorized(hook);
      stubFetch(() => json(401, { error: "UNAUTHORIZED", message: "no session" }));
      await expect(api("/reports")).rejects.toMatchObject({ status: 401 });
      expect(hook).toHaveBeenCalledOnce();
    });

    it("stays quiet for silent401 and for a failed login", async () => {
      const hook = vi.fn();
      setOnUnauthorized(hook);
      stubFetch(() => json(401, { message: "x" }));
      await expect(api("/auth/me", { silent401: true })).rejects.toBeInstanceOf(ApiError);
      stubFetch(
        () => json(200, { csrf_token: "t" }),
        () => json(401, { message: "invalid credentials" }),
      );
      await expect(api("/auth/login", { method: "POST", body: {} })).rejects.toBeInstanceOf(ApiError);
      expect(hook).not.toHaveBeenCalled();
    });
  });

  it("fails when the CSRF token cannot be fetched", async () => {
    stubFetch(() => json(500, {}));
    await expect(api("/reports", { method: "POST", body: {} })).rejects.toMatchObject({ status: 500, code: "CSRF" });
  });
});
