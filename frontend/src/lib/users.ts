import { api } from "./api";
import type { Page, ResetPasswordInput, Role, User, UserInput, UserUpdate } from "./api-schema";

export type { Role, User, UserInput, UserUpdate };

// A bare array capped at 100: the endpoint has no page or total.
// status is one of the four, or empty / "all" for everyone; the server filters (the role filter does not).
export function listUsers(email = "", status = ""): Promise<User[]> {
  const qs = new URLSearchParams({ limit: "100" });
  if (email) qs.set("email", email);
  if (status && status !== "all") qs.set("status", status);
  return api<User[]>(`/users?${qs}`);
}

export const getUser = (id: number) => api<User>(`/admin/users/${id}`);

export const createUser = (input: UserInput) => api<User>("/admin/users", { method: "POST", body: input });

// PUT replaces name, area and church; the email, password and role are not changed.
export const updateUser = (id: number, input: UserUpdate) =>
  api<User>(`/admin/users/${id}`, { method: "PUT", body: input });

// Moving a user away from active also ends their sessions on the server (D-A10).
export const changeUserStatus = (id: number, status: string) =>
  api<User>(`/admin/users/${id}/status`, { method: "PUT", body: { status } });

// Sets another user's password (D-A12): ends all of their sessions and clears their lockout. Answers 204.
export const resetUserPassword = (id: number, new_password: string, confirm_password: string) =>
  api<void>(`/admin/users/${id}/reset-password`, {
    method: "POST",
    body: { new_password, confirm_password } satisfies ResetPasswordInput,
  });

// The handler answers { data, meta }; the Swagger type says a bare array, which is wrong.
export async function listRoles(): Promise<Role[]> {
  return (await api<Page<Role>>("/admin/roles?limit=100")).data;
}
