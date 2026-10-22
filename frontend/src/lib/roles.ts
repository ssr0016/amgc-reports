import { api } from "./api";
import type { Page, PermissionInfo, RoleDetail, RoleInput, User } from "./api-schema";

export type { PermissionInfo, RoleDetail, RoleInput };

// Mirrors accesscontrol.BuiltInRoles: these are read-only (D-A21).
export const BUILT_IN_ROLES = ["admin", "editor", "user"] as const;
export const isBuiltIn = (name: string) => (BUILT_IN_ROLES as readonly string[]).includes(name);

// The handler answers { data, meta }; 100 is the server's cap and far above the permission count.
export async function listPermissions(): Promise<PermissionInfo[]> {
  return (await api<Page<PermissionInfo>>("/admin/permissions?limit=100")).data;
}

export const getRole = (id: number) => api<RoleDetail>(`/admin/roles/${id}`);

export const createRole = (input: RoleInput) => api<RoleDetail>("/admin/roles", { method: "POST", body: input });

// PUT replaces the name, description and the whole permission set.
export const updateRole = (id: number, input: RoleInput) =>
  api<RoleDetail>(`/admin/roles/${id}`, { method: "PUT", body: input });

export const deleteRole = (id: number) => api<void>(`/admin/roles/${id}`, { method: "DELETE" });

export const changeUserRole = (userId: number, roleId: number) =>
  api<User>(`/admin/users/${userId}/role`, { method: "PUT", body: { role_id: roleId } });

// Frontend wording only: the server never sees these labels.
const GROUP: Record<string, string> = { reports: "Reports", users: "Users", roles: "Admin", permissions: "Admin" };
const ORDER = ["Reports", "Users", "Admin"];
const ACTION: Record<string, string> = {
  read: "View",
  read_any: "View anyone's",
  write: "Create and edit",
  write_any: "Edit anyone's",
  delete: "Delete",
  delete_any: "Delete anyone's",
  approve: "Approve",
};

const capitalize = (s: string) => s.charAt(0).toUpperCase() + s.slice(1);

export const actionWords = (action: string) => ACTION[action] ?? capitalize(action.replaceAll("_", " "));

// One group per category, Reports, Users and Admin first, then any other resource by name.
export function groupPermissions(perms: PermissionInfo[]): { label: string; permissions: PermissionInfo[] }[] {
  const groups = new Map<string, PermissionInfo[]>();
  for (const perm of [...perms].sort((a, b) => a.name.localeCompare(b.name))) {
    const label = GROUP[perm.resource] ?? capitalize(perm.resource);
    groups.set(label, [...(groups.get(label) ?? []), perm]);
  }
  const rank = (l: string) => (ORDER.includes(l) ? ORDER.indexOf(l) : ORDER.length);
  return [...groups]
    .sort(([a], [b]) => rank(a) - rank(b) || a.localeCompare(b))
    .map(([label, permissions]) => ({ label, permissions }));
}
