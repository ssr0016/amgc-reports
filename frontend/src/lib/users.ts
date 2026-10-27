import { api } from "./api";
import type { Page, ResetPasswordInput, Role, RoleDetail, User, UserInput, UserUpdate } from "./api-schema";

export type { Role, User, UserInput, UserUpdate };

// One page of users and the total that match; 100 is the server's cap on limit.
// status is one of the four, "deleted", or empty / "all" for everyone not deleted; the server filters, and by roleId when given.
export function listUsers(email = "", status = "", roleId?: number, page = 1, limit = 100): Promise<Page<User>> {
  const qs = new URLSearchParams({ limit: String(limit) });
  if (page > 1) qs.set("page", String(page));
  if (email) qs.set("email", email);
  if (roleId) qs.set("role_id", String(roleId));
  if (status && status !== "all") qs.set("status", status);
  return api<Page<User>>(`/users?${qs}`);
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

// Soft delete (D-A11): the user cannot sign in and their sessions end. The reason is optional.
export const deleteUser = (id: number, reason: string) =>
  api<User>(`/admin/users/${id}`, { method: "DELETE", body: { reason } });

export const restoreUser = (id: number) => api<User>(`/admin/users/${id}/restore`, { method: "POST" });

// The handler answers { data, meta }; the Swagger type says a bare array, which is wrong.
// Each role carries its permissions and user_count.
export async function listRoles(): Promise<RoleDetail[]> {
  return (await api<Page<RoleDetail>>("/admin/roles?limit=100")).data;
}
