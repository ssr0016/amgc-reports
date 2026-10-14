import { api } from "./api";
import type { Page, Role, User, UserInput, UserUpdate } from "./api-schema";

export type { Role, User, UserInput, UserUpdate };

// A bare array capped at 100: the endpoint has no page or total.
export function listUsers(email = ""): Promise<User[]> {
  const qs = new URLSearchParams({ limit: "100" });
  if (email) qs.set("email", email);
  return api<User[]>(`/users?${qs}`);
}

export const getUser = (id: number) => api<User>(`/admin/users/${id}`);

export const createUser = (input: UserInput) => api<User>("/admin/users", { method: "POST", body: input });

// PUT replaces name, area and church; the email, password and role are not changed.
export const updateUser = (id: number, input: UserUpdate) =>
  api<User>(`/admin/users/${id}`, { method: "PUT", body: input });

// The handler answers { data, meta }; the Swagger type says a bare array, which is wrong.
export async function listRoles(): Promise<Role[]> {
  return (await api<Page<Role>>("/admin/roles?limit=100")).data;
}
