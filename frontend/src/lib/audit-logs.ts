import { api } from "./api";
import type { AuditLog, Page } from "./api-schema";

export type { AuditLog };

// The Logs tab's page size; the server defaults to it too.
const PAGE_SIZE = 50;

export type AuditLogFilters = { month: string; year: number; action?: string; page?: number };

export function listAuditLogs({ month, year, action, page }: AuditLogFilters): Promise<Page<AuditLog>> {
  const qs = new URLSearchParams({ month, year: String(year) });
  if (action) qs.set("action", action);
  if (page && page > 1) qs.set("page", String(page));
  qs.set("limit", String(PAGE_SIZE));
  return api<Page<AuditLog>>(`/admin/audit-logs?${qs}`);
}

// The badge colour groups actions; the label is always printed, so colour is never the only cue.
type Group = "create" | "change" | "delete" | "approve" | "download" | "security" | "warning";

// Every action the server audits, in the order of the Action filter.
export const ACTIONS: readonly { code: string; label: string; group: Group }[] = [
  { code: "CREATE_USER", label: "Created user", group: "create" },
  { code: "UPDATE_USER", label: "Updated user", group: "change" },
  { code: "DELETE_USER", label: "Deleted user", group: "delete" },
  { code: "RESTORE_USER", label: "Restored user", group: "delete" },
  { code: "CHANGE_USER_STATUS", label: "Changed user status", group: "change" },
  { code: "ADMIN_RESET_PASSWORD", label: "Reset password", group: "change" },
  { code: "CHANGE_USER_ROLE", label: "Changed user role", group: "change" },
  { code: "CREATE_ROLE", label: "Created role", group: "create" },
  { code: "UPDATE_ROLE", label: "Updated role", group: "change" },
  { code: "DELETE_ROLE", label: "Deleted role", group: "delete" },
  { code: "APPROVE_REPORT", label: "Approved report", group: "approve" },
  { code: "UNAPPROVE_REPORT", label: "Unapproved report", group: "approve" },
  { code: "DOWNLOAD_SINGLE_EXCEL", label: "Downloaded Excel", group: "download" },
  { code: "DOWNLOAD_BULK_EXCEL", label: "Downloaded Excel (bulk)", group: "download" },
  { code: "CREATE_REPORT", label: "Created report", group: "create" },
  { code: "UPDATE_REPORT", label: "Updated report", group: "change" },
  { code: "DELETE_REPORT", label: "Deleted report", group: "delete" },
  { code: "CHANGE_PASSWORD", label: "Changed own password", group: "change" },
  { code: "LOGIN", label: "Logged in", group: "security" },
  { code: "LOGOUT", label: "Logged out", group: "security" },
  { code: "LOGIN_FAILED", label: "Failed login", group: "warning" },
];

const byCode = new Map(ACTIONS.map((a) => [a.code, a]));

// An action this build does not know yet (the API shipped first) shows as its raw code.
export const actionLabel = (code: string) => byCode.get(code)?.label ?? code;
export const actionGroup = (code: string): Group | undefined => byCode.get(code)?.group;

const REASONS: Record<string, string> = {
  unknown_account: "Unknown account",
  wrong_password: "Wrong password",
  locked: "Account locked",
  inactive: "Account inactive",
  disciplinary: "Account under review",
  deceased: "Account deceased",
};

const capitalize = (s: string) => s.charAt(0).toUpperCase() + s.slice(1);
const text = (v: unknown) => (v === null || v === undefined || v === "" ? "—" : String(v));
// A list of permission names, or a dash when there are none.
const list = (v: unknown) => (Array.isArray(v) && v.length > 0 ? v.join(", ") : "—");

// The details of one row as labelled values. A known action shows only the keys it has a
// label for (never a raw JSON blob); an unknown one lists its keys as they are.
export function detailRows(action: string, details: Record<string, unknown>): [string, string][] {
  if (!byCode.has(action)) return Object.entries(details).map(([k, v]) => [k, text(v)]);
  const d = details;
  const rows: [string, string][] = [];
  if (d.month !== undefined && d.year !== undefined) rows.push(["Period", `${d.month} ${d.year}`]);
  if (action === "DOWNLOAD_BULK_EXCEL" && d.count !== undefined) rows.push(["Reports", text(d.count)]);
  if (d.role_id !== undefined) rows.push(["Role ID", text(d.role_id)]);
  // Role names are shown as typed; a status is a lowercase code.
  if (d.from !== undefined && d.to !== undefined) {
    if (action === "UPDATE_ROLE") rows.push(["Name", `${text(d.from)} → ${text(d.to)}`]);
    else if (action === "CHANGE_USER_ROLE") rows.push(["Role", `${text(d.from)} → ${text(d.to)}`]);
    else rows.push(["Status", `${capitalize(String(d.from))} → ${capitalize(String(d.to))}`]);
  }
  if (d.name !== undefined) rows.push(["Role", text(d.name)]);
  if (d.permissions !== undefined) rows.push(["Permissions", list(d.permissions)]);
  if (d.added !== undefined) rows.push(["Permissions added", list(d.added)]);
  if (d.removed !== undefined) rows.push(["Permissions removed", list(d.removed)]);
  if ("reason" in d) {
    rows.push(["Reason", action === "LOGIN_FAILED" ? (REASONS[String(d.reason)] ?? text(d.reason)) : text(d.reason)]);
  }
  if (d.deleted_at !== undefined) rows.push(["Deleted at", d.deleted_at ? formatTimestamp(String(d.deleted_at), true) : "—"]);
  if (d.deleted_by !== undefined) rows.push(["Deleted by (user ID)", text(d.deleted_by)]);
  if ("deleted_reason" in d) rows.push(["Deletion reason", text(d.deleted_reason)]);
  const sessions = d.sessions_ended ?? d.sessions_revoked;
  if (sessions !== undefined) rows.push(["Sessions ended", text(sessions)]);
  return rows;
}

// Audit times are shown in Asia/Manila, the zone the month filter counts in.
export function formatTimestamp(iso: string, full = false) {
  const s = new Intl.DateTimeFormat("en-US", {
    timeZone: "Asia/Manila",
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
    ...(full ? { second: "2-digit" } : {}),
  }).format(new Date(iso));
  return full ? `${s} (Asia/Manila)` : s;
}
