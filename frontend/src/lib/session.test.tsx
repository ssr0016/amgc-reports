import { act, render, renderHook, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { resetCsrf } from "./api";
import { QueryProvider } from "./query";
import { SessionProvider, useSession } from "./session";

const toast = vi.hoisted(() => ({ success: vi.fn(), info: vi.fn(), error: vi.fn() }));
vi.mock("sonner", () => ({ toast }));

const me = (role: string, permissions: string[]) => ({ id: 1, name: "Ana", role, permissions });
const json = (status: number, body: unknown) => new Response(JSON.stringify(body), { status });

// A refresh that finds no sign-in; every test that can see a 401 gets it.
const NO_SESSION = {
  "/auth/csrf": () => json(200, { csrf_token: "t" }),
  "/auth/refresh": () => json(401, {}),
};

// Answers by path so the order of calls does not matter.
function stubApi(routes: Record<string, () => Response>) {
  const f = vi.fn(async (url: string) => {
    const path = String(url).replace(/^undefined/, "");
    const route = routes[path];
    if (!route) throw new Error(`unexpected ${path}`);
    return route();
  });
  vi.stubGlobal("fetch", f);
  return f;
}

const wrapper = ({ children }: { children: React.ReactNode }) => <SessionProvider>{children}</SessionProvider>;

beforeEach(() => {
  resetCsrf();
  vi.clearAllMocks();
});
afterEach(() => vi.unstubAllGlobals());

describe("SessionProvider", () => {
  it("starts loading, then authenticated with permissions from /auth/me", async () => {
    stubApi({ "/auth/me": () => json(200, me("editor", ["reports:read"])) });
    const { result } = renderHook(() => useSession(), { wrapper });
    expect(result.current.status).toBe("loading");
    expect(result.current.can("reports:read")).toBe(false);
    await waitFor(() => expect(result.current.status).toBe("authenticated"));
    expect(result.current.can("reports:read")).toBe(true);
    expect(result.current.can("reports:approve")).toBe(false);
    expect(result.current.isAdmin).toBe(false);
  });

  it("is unauthenticated on 401 and on any other failure", async () => {
    stubApi({ ...NO_SESSION, "/auth/me": () => json(401, {}) });
    const a = renderHook(() => useSession(), { wrapper });
    await waitFor(() => expect(a.result.current.status).toBe("unauthenticated"));

    stubApi({ "/auth/me": () => json(500, {}) });
    const b = renderHook(() => useSession(), { wrapper });
    await waitFor(() => expect(b.result.current.status).toBe("unauthenticated"));
  });

  it("isAdmin follows the role", async () => {
    stubApi({ "/auth/me": () => json(200, me("admin", [])) });
    const { result } = renderHook(() => useSession(), { wrapper });
    await waitFor(() => expect(result.current.isAdmin).toBe(true));
  });

  it("canAdmin is true for the admin role or any admin-area permission, never for a plain user's", async () => {
    const cases: [string, string[], boolean][] = [
      ["admin", [], true],
      ["Reviewer", ["tracker:read"], true],
      ["Reviewer", ["reports:approve"], true],
      ["user", ["users:read", "reports:read", "reports:read_any", "reports:write", "reports:delete"], false],
      ["editor", ["users:read", "users:write", "reports:read_any", "reports:write_any"], false],
    ];
    for (const [role, perms, want] of cases) {
      stubApi({ "/auth/me": () => json(200, me(role, perms)) });
      const { result, unmount } = renderHook(() => useSession(), { wrapper });
      await waitFor(() => expect(result.current.status).toBe("authenticated"));
      expect(result.current.canAdmin, `${role} ${perms}`).toBe(want);
      unmount();
    }
  });

  it("signIn loads the user and signOut clears it, even if logout fails", async () => {
    let signedIn = false;
    stubApi({
      ...NO_SESSION,
      "/auth/me": () => (signedIn ? json(200, me("user", ["reports:read"])) : json(401, {})),
      "/auth/csrf": () => json(200, { csrf_token: "t" }),
      "/auth/login": () => {
        signedIn = true;
        return json(200, {});
      },
      "/auth/logout": () => json(500, {}),
    });
    const { result } = renderHook(() => useSession(), { wrapper });
    await waitFor(() => expect(result.current.status).toBe("unauthenticated"));

    await act(() => result.current.signIn("a@b.c", "pw"));
    expect(result.current.status).toBe("authenticated");
    expect(result.current.user?.name).toBe("Ana");
    expect(toast.success).toHaveBeenCalledWith("Welcome back, Ana!");
    await waitFor(() => expect(document.cookie).toContain("signed_in=1"));

    await act(() => result.current.signOut().catch(() => {}));
    expect(result.current.status).toBe("unauthenticated");
    expect(result.current.user).toBeNull();
    expect(toast.info).toHaveBeenCalledWith("You have been logged out.");
    await waitFor(() => expect(document.cookie).not.toContain("signed_in"));
  });

  it("a 401 mid-session signs the user out", async () => {
    stubApi({
      ...NO_SESSION,
      "/auth/me": () => json(200, me("user", [])),
      "/reports": () => json(401, {}),
    });
    const { result } = renderHook(() => useSession(), { wrapper });
    await waitFor(() => expect(result.current.status).toBe("authenticated"));
    const { api } = await import("./api");
    await act(() => api("/reports").catch(() => {}));
    expect(result.current.status).toBe("unauthenticated");
    expect(toast.error).toHaveBeenCalledWith("Your session has expired. Please log in again.", { id: "session-expired" });
  });

  it("useSession outside a provider is an error", () => {
    expect(() => renderHook(() => useSession())).toThrow(/inside <SessionProvider>/);
  });
});

describe("QueryProvider", () => {
  it("renders its children", () => {
    render(<QueryProvider>hello</QueryProvider>);
    expect(screen.getByText("hello")).toBeInTheDocument();
  });
});

it("updates only the matching signed-in user's avatar", async () => {
  stubApi({ "/auth/me": () => json(200, { ...me("user", []), user_id: 7, avatar_url: "" }) });
  const { result } = renderHook(() => useSession(), { wrapper });
  await waitFor(() => expect(result.current.status).toBe("authenticated"));
  act(() => result.current.updateAvatar(7, "https://cdn.example/a"));
  expect(result.current.user?.avatar_url).toBe("https://cdn.example/a");
  expect(result.current.user?.name).toBe("Ana");
  act(() => result.current.updateAvatar(9, "https://cdn.example/other"));
  expect(result.current.user?.avatar_url).toBe("https://cdn.example/a");
});
