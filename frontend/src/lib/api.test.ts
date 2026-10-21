import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { api, ApiError, downloadFile, resetCsrf, setOnUnauthorized } from "./api";

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
    it("calls the unauthorized hook when the refresh fails too", async () => {
      const hook = vi.fn();
      setOnUnauthorized(hook);
      stubFetch(
        () => json(401, { error: "UNAUTHORIZED", message: "no session" }),
        () => json(200, { csrf_token: "t" }),
        () => json(401, { error: "UNAUTHORIZED", message: "refresh token expired" }),
      );
      await expect(api("/reports")).rejects.toMatchObject({ status: 401 });
      expect(hook).toHaveBeenCalledOnce();
    });

    it("stays quiet for silent401 and for a failed login", async () => {
      const hook = vi.fn();
      setOnUnauthorized(hook);
      stubFetch(
        () => json(401, { message: "x" }),
        () => json(200, { csrf_token: "t" }),
        () => json(401, { message: "x" }),
      );
      await expect(api("/auth/me", { silent401: true })).rejects.toBeInstanceOf(ApiError);
      resetCsrf(); // the refresh above fetched one
      const f = stubFetch(
        () => json(200, { csrf_token: "t" }),
        () => json(401, { message: "invalid credentials" }),
      );
      await expect(api("/auth/login", { method: "POST", body: {} })).rejects.toBeInstanceOf(ApiError);
      expect(hook).not.toHaveBeenCalled();
      expect(f.mock.calls.map(pathOf), "a failed login must not try to refresh").toEqual(["/auth/csrf", "/auth/login"]);
    });
  });

  describe("refresh on 401", () => {
    it("refreshes once and replays the request", async () => {
      const hook = vi.fn();
      setOnUnauthorized(hook);
      const f = stubFetch(
        () => json(401, { message: "expired" }),
        () => json(200, { csrf_token: "t" }),
        () => new Response(null, { status: 204 }), // /auth/refresh
        () => json(200, { ok: 1 }),
      );
      expect(await api("/reports")).toEqual({ ok: 1 });
      expect(f.mock.calls.map(pathOf)).toEqual(["/reports", "/auth/csrf", "/auth/refresh", "/reports"]);
      expect(f.mock.calls[2][1]).toMatchObject({ method: "POST" });
      expect(headersOf(f.mock.calls[2])["X-CSRF-Token"]).toBe("t");
      expect(hook).not.toHaveBeenCalled();
    });

    it("replays a write with its body", async () => {
      const f = stubFetch(
        () => json(200, { csrf_token: "t" }),
        () => json(401, {}),
        () => new Response(null, { status: 204 }),
        () => json(201, { id: 5 }),
      );
      expect(await api("/reports", { method: "POST", body: { month: "May" } })).toEqual({ id: 5 });
      expect(f.mock.calls.map(pathOf)).toEqual(["/auth/csrf", "/reports", "/auth/refresh", "/reports"]);
      expect(f.mock.calls[3][1]).toMatchObject({ body: '{"month":"May"}' });
    });

    it("replays only once: a second 401 signs out", async () => {
      const hook = vi.fn();
      setOnUnauthorized(hook);
      const f = stubFetch(
        () => json(401, {}),
        () => json(200, { csrf_token: "t" }),
        () => new Response(null, { status: 204 }),
        () => json(401, {}),
      );
      await expect(api("/reports")).rejects.toMatchObject({ status: 401 });
      expect(f).toHaveBeenCalledTimes(4);
      expect(hook).toHaveBeenCalledOnce();
    });

    it("concurrent 401s share one refresh (single flight)", async () => {
      const hook = vi.fn();
      setOnUnauthorized(hook);
      let refreshed = false;
      let refreshCalls = 0;
      const fetchMock = vi.fn(async (url: string) => {
        const path = String(url).replace(/^undefined/, "");
        if (path === "/auth/csrf") return json(200, { csrf_token: "t" });
        if (path === "/auth/refresh") {
          refreshCalls++;
          await new Promise((r) => setTimeout(r, 10)); // slow enough for the others to pile up
          refreshed = true;
          return new Response(null, { status: 204 });
        }
        return refreshed ? json(200, { path }) : json(401, {});
      });
      vi.stubGlobal("fetch", fetchMock);

      const results = await Promise.all([api("/reports"), api("/users"), api("/stats")]);

      expect(results).toEqual([{ path: "/reports" }, { path: "/users" }, { path: "/stats" }]);
      expect(refreshCalls).toBe(1);
      expect(hook).not.toHaveBeenCalled();
    });

    it("a later 401 starts a new refresh", async () => {
      const f = stubFetch(
        () => json(401, {}),
        () => json(200, { csrf_token: "t" }),
        () => new Response(null, { status: 204 }),
        () => json(200, { n: 1 }),
        () => json(401, {}),
        () => new Response(null, { status: 204 }),
        () => json(200, { n: 2 }),
      );
      expect(await api("/a")).toEqual({ n: 1 });
      expect(await api("/b")).toEqual({ n: 2 });
      expect(f.mock.calls.map(pathOf).filter((p) => p === "/auth/refresh")).toHaveLength(2);
    });

    it("does not refresh for logout or for the refresh call itself", async () => {
      const hook = vi.fn();
      setOnUnauthorized(hook);
      const f = stubFetch(
        () => json(200, { csrf_token: "t" }),
        () => json(401, {}),
      );
      await expect(api("/auth/refresh", { method: "POST" })).rejects.toMatchObject({ status: 401 });
      expect(f.mock.calls.map(pathOf)).toEqual(["/auth/csrf", "/auth/refresh"]);
      expect(hook).toHaveBeenCalledOnce();
    });

    it("a network error during the refresh leaves the 401 to decide", async () => {
      const hook = vi.fn();
      setOnUnauthorized(hook);
      const fetchMock = vi
        .fn()
        .mockResolvedValueOnce(json(401, {}))
        .mockResolvedValueOnce(json(200, { csrf_token: "t" }))
        .mockRejectedValueOnce(new TypeError("Failed to fetch"));
      vi.stubGlobal("fetch", fetchMock);
      await expect(api("/reports")).rejects.toMatchObject({ status: 401 });
      expect(hook).toHaveBeenCalledOnce();
    });
  });

  it("fails when the CSRF token cannot be fetched", async () => {
    stubFetch(() => json(500, {}));
    await expect(api("/reports", { method: "POST", body: {} })).rejects.toMatchObject({ status: 500, code: "CSRF" });
  });
});

describe("downloadFile", () => {
  const xlsx = (disposition: string) =>
    new Response("PK", { status: 200, headers: { "Content-Disposition": disposition } });

  it("POSTs with the CSRF token and returns the blob and the UTF-8 filename", async () => {
    const f = stubFetch(
      () => json(200, { csrf_token: "t" }),
      () => xlsx(`attachment; filename="Caf_ May 2026.xlsx"; filename*=UTF-8''Caf%C3%A9%20May%202026.xlsx`),
    );
    const got = await downloadFile("/admin/reports/excel", { month: "May", year: 2026 });
    expect(got.filename).toBe("Café May 2026.xlsx");
    expect(await got.blob.text()).toBe("PK");
    expect(f.mock.calls[1][1]).toMatchObject({ method: "POST", body: '{"month":"May","year":2026}' });
    expect(headersOf(f.mock.calls[1])["X-CSRF-Token"]).toBe("t");
  });

  it("falls back to the ASCII name, then to a generic one", async () => {
    stubFetch(
      () => json(200, { csrf_token: "t" }),
      () => xlsx(`attachment; filename="Report 4 May 2026.xlsx"; filename*=UTF-8''%E0%A4%A`),
      () => xlsx(""),
    );
    expect((await downloadFile("/admin/reports/4/excel")).filename).toBe("Report 4 May 2026.xlsx");
    expect((await downloadFile("/admin/reports/4/excel")).filename).toBe("download");
  });

  it("throws the server's error", async () => {
    stubFetch(
      () => json(200, { csrf_token: "t" }),
      () => json(404, { error: "NOT_FOUND", message: "No reports for that month." }),
    );
    await expect(downloadFile("/admin/reports/excel", {})).rejects.toMatchObject({ status: 404, message: "No reports for that month." });
  });
});
