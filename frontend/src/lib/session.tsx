"use client";

import { createContext, useContext, useEffect, useState } from "react";
import { resetCsrf, setOnUnauthorized } from "./api";
import { getCurrentUser, login, logout, type Me } from "./auth";

type Status = "loading" | "authenticated" | "unauthenticated";

type Session = {
  user: Me | null;
  status: Status;
  signIn: (email: string, password: string) => Promise<void>;
  signOut: () => Promise<void>;
};

const SessionContext = createContext<Session | null>(null);

export function useSession(): Session {
  const ctx = useContext(SessionContext);
  if (!ctx) throw new Error("useSession must be used inside <SessionProvider>");
  return ctx;
}

// Asks /auth/me once at app start. Redirects are left to the route guard.
export function SessionProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<Me | null>(null);
  const [status, setStatus] = useState<Status>("loading");

  useEffect(() => {
    getCurrentUser()
      .then((me) => {
        setUser(me);
        setStatus(me ? "authenticated" : "unauthenticated");
      })
      // shortcut: any failure (network, 5xx) reads as signed out; F10 adds a retry state.
      .catch(() => setStatus("unauthenticated"));

    // A 401 mid-session: the session is gone.
    setOnUnauthorized(() => {
      resetCsrf();
      setUser(null);
      setStatus("unauthenticated");
    });
    return () => setOnUnauthorized(null);
  }, []);

  async function signIn(email: string, password: string) {
    setUser(await login(email, password));
    setStatus("authenticated");
  }

  async function signOut() {
    try {
      await logout();
    } finally {
      setUser(null);
      setStatus("unauthenticated");
    }
  }

  return (
    <SessionContext.Provider value={{ user, status, signIn, signOut }}>
      {children}
    </SessionContext.Provider>
  );
}
