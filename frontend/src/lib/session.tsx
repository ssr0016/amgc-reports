"use client";

import { createContext, useContext, useEffect, useRef, useState } from "react";
import { toast } from "sonner";
import { resetCsrf, setOnUnauthorized } from "./api";
import { SIGNED_IN_COOKIE } from "./routes";
import { queryClient } from "./query";
import { getCurrentUser, login, logout, type Me } from "./auth";
import { ADMIN_AREA, type Permission } from "./permissions";

type Status = "loading" | "authenticated" | "unauthenticated";

type Session = {
  user: Me | null;
  status: Status;
  // False while loading or signed out.
  can: (permission: Permission) => boolean;
  // False while loading or signed out. Gates the admin screens; the server still enforces the role.
  isAdmin: boolean;
  // The admin role, or any admin-area permission (37b): may open /admin.
  canAdmin: boolean;
  signIn: (email: string, password: string) => Promise<void>;
  signOut: () => Promise<void>;
};

const SessionContext = createContext<Session | null>(null);

// A hint for proxy.ts, which cannot see the API's session (the access token expires in 10 minutes
// and the refresh token is scoped to /api/v1/auth). Not a credential: the API still checks every request.
function setSignedInCookie(on: boolean) {
  const secure = location.protocol === "https:" ? "; secure" : "";
  document.cookie = on
    ? `${SIGNED_IN_COOKIE}=1; path=/; max-age=86400; samesite=lax${secure}`
    : `${SIGNED_IN_COOKIE}=; path=/; max-age=0`;
}

export function useSession(): Session {
  const ctx = useContext(SessionContext);
  if (!ctx) throw new Error("useSession must be used inside <SessionProvider>");
  return ctx;
}

// Asks /auth/me once at app start. Redirects are left to the route guard.
export function SessionProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<Me | null>(null);
  const [status, setStatus] = useState<Status>("loading");
  // Read by the 401 handler below, which is registered once.
  const statusRef = useRef(status);

  // Every status change goes through here, so the signed_in cookie is written before the change
  // renders: an effect would run after the login page's redirect, and proxy.ts would bounce it.
  function settle(next: Exclude<Status, "loading">, me: Me | null) {
    setSignedInCookie(next === "authenticated");
    statusRef.current = next;
    setUser(me);
    setStatus(next);
  }

  useEffect(() => {
    getCurrentUser()
      .then((me) => settle(me ? "authenticated" : "unauthenticated", me))
      // shortcut: any failure (network, 5xx) reads as signed out; F10 adds a retry state.
      .catch(() => settle("unauthenticated", null));

    // A 401 mid-session: the session is gone.
    setOnUnauthorized(() => {
      if (statusRef.current === "authenticated") {
        toast.error("Your session has expired. Please log in again.", { id: "session-expired" });
      }
      resetCsrf();
      queryClient.clear();
      settle("unauthenticated", null);
    });
    return () => setOnUnauthorized(null);
  }, []);

  async function signIn(email: string, password: string) {
    const me = await login(email, password);
    settle("authenticated", me);
    toast.success(`Welcome back, ${me.name}!`);
  }

  async function signOut() {
    try {
      await logout();
    } finally {
      queryClient.clear();
      settle("unauthenticated", null);
      toast.info("You have been logged out.");
    }
  }

  const can = (permission: Permission) => !!user?.permissions.includes(permission);
  const isAdmin = user?.role === "admin";
  const canAdmin = isAdmin || ADMIN_AREA.some(can);

  return (
    <SessionContext.Provider value={{ user, status, can, isAdmin, canAdmin, signIn, signOut }}>
      {children}
    </SessionContext.Provider>
  );
}
