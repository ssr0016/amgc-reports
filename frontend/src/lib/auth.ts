import { api, ApiError, resetCsrf } from "./api";
import type { Me } from "./api-schema";

export type { Me };

// The session is renewed on login/logout, so the CSRF token is refetched lazily.
export async function login(email: string, password: string): Promise<Me> {
  await api("/auth/login", { method: "POST", body: { email, password } });
  resetCsrf();
  return api<Me>("/auth/me");
}

export async function logout(): Promise<void> {
  try {
    await api("/auth/logout", { method: "POST" });
  } finally {
    resetCsrf();
  }
}

// Returns null when signed out (401). The 401 is the expected answer here, so it
// does not fire the unauthorized hook.
export async function getCurrentUser(): Promise<Me | null> {
  try {
    return await api<Me>("/auth/me", { silent401: true });
  } catch (e) {
    if (e instanceof ApiError && e.status === 401) return null;
    throw e;
  }
}
