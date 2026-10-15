import { act, render, renderHook, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { resetCsrf } from "./api";
import { QueryProvider } from "./query";
import { SessionProvider, useSession } from "./session";

const me = (role: string, permissions: string[]) => ({ id: 1, name: "Ana", role, permissions });
const json = (status: number, body: unknown) => new Response(JSON.stringify(body), { status });

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

beforeEach(() => resetCsrf());
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
    stubApi({ "/auth/me": () => json(401, {}) });
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

  it("signIn loads the user and signOut clears it, even if logout fails", async () => {
    let signedIn = false;
    stubApi({
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

    await act(() => result.current.signOut().catch(() => {}));
    expect(result.current.status).toBe("unauthenticated");
    expect(result.current.user).toBeNull();
  });

  it("a 401 mid-session signs the user out", async () => {
    stubApi({
      "/auth/me": () => json(200, me("user", [])),
      "/reports": () => json(401, {}),
    });
    const { result } = renderHook(() => useSession(), { wrapper });
    await waitFor(() => expect(result.current.status).toBe("authenticated"));
    const { api } = await import("./api");
    await act(() => api("/reports").catch(() => {}));
    expect(result.current.status).toBe("unauthenticated");
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
