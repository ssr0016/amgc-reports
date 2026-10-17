import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { resetCsrf } from "./api";
import { changePassword, getCurrentUser, login, logout } from "./auth";

const json = (status: number, body: unknown) => new Response(JSON.stringify(body), { status });
const paths = (f: ReturnType<typeof vi.fn>) => f.mock.calls.map((c) => String(c[0]).replace(/^undefined/, ""));

beforeEach(() => resetCsrf());
afterEach(() => vi.unstubAllGlobals());

describe("auth", () => {
  it("login drops the CSRF token so the renewed session gets a fresh one", async () => {
    const f = vi.fn()
      .mockResolvedValueOnce(json(200, { csrf_token: "a" }))
      .mockResolvedValueOnce(json(200, {}))
      .mockResolvedValueOnce(json(200, { id: 1 }))
      .mockResolvedValueOnce(json(200, { csrf_token: "b" }))
      .mockResolvedValueOnce(json(200, {}));
    vi.stubGlobal("fetch", f);
    await expect(login("a@b.c", "pw")).resolves.toEqual({ id: 1 });
    await logout();
    expect(paths(f)).toEqual(["/auth/csrf", "/auth/login", "/auth/me", "/auth/csrf", "/auth/logout"]);
  });

  it("logout drops the token even when the request fails", async () => {
    const f = vi.fn()
      .mockResolvedValueOnce(json(200, { csrf_token: "a" }))
      .mockResolvedValueOnce(json(500, {}))
      .mockResolvedValueOnce(json(200, { csrf_token: "b" }))
      .mockResolvedValueOnce(json(200, {}));
    vi.stubGlobal("fetch", f);
    await expect(logout()).rejects.toMatchObject({ status: 500 });
    await logout();
    expect(paths(f)).toContain("/auth/logout");
    expect(paths(f).filter((p) => p === "/auth/csrf")).toHaveLength(2);
  });

  it("getCurrentUser is null on 401 and rethrows other errors", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(json(401, {})));
    await expect(getCurrentUser()).resolves.toBeNull();
    vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(json(500, {})));
    await expect(getCurrentUser()).rejects.toMatchObject({ status: 500 });
    vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(json(200, { id: 3 })));
    await expect(getCurrentUser()).resolves.toEqual({ id: 3 });
  });

  it("changePassword posts both passwords and drops the CSRF token", async () => {
    const f = vi
      .fn()
      .mockResolvedValueOnce(json(200, { csrf_token: "a" }))
      .mockResolvedValueOnce(new Response(null, { status: 204 }))
      .mockResolvedValueOnce(json(200, { csrf_token: "b" }))
      .mockResolvedValueOnce(new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", f);
    await expect(changePassword("old", "new", "new")).resolves.toBeUndefined();
    expect(JSON.parse(f.mock.calls[1][1].body)).toEqual({ current_password: "old", new_password: "new", confirm_password: "new" });
    await changePassword("old", "new", "new");
    expect(paths(f)).toEqual(["/auth/csrf", "/auth/change-password", "/auth/csrf", "/auth/change-password"]);
  });
});
