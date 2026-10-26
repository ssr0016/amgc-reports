import { api } from "./api";
import type { Tracker, TrackerRow } from "./api-schema";

export type { Tracker, TrackerRow };

export const getTracker = (month: string, year: number) =>
  api<Tracker>(`/admin/tracker?${new URLSearchParams({ month, year: String(year) })}`);

export const SUBMISSION_STATUSES = [
  { code: "submitted", label: "Submitted" },
  { code: "no_report", label: "No Report" },
] as const;

export const REVIEW_STATUSES = [
  { code: "approved", label: "Approved" },
  { code: "pending", label: "Pending" },
  { code: "needs_revision", label: "Needs Revision" },
] as const;

// A value this code does not know is shown as it came, not hidden.
export const trackerLabel = (status: string) =>
  [...SUBMISSION_STATUSES, ...REVIEW_STATUSES].find((s) => s.code === status)?.label ?? status;

export function trackerCounts(rows: readonly TrackerRow[]) {
  const n = (status: string) => rows.filter((r) => r.review_status === status).length;
  return {
    expected: rows.length,
    submitted: rows.filter((r) => r.submission_status === "submitted").length,
    noReport: rows.filter((r) => r.submission_status === "no_report").length,
    approved: n("approved"),
    pending: n("pending"),
    needsRevision: n("needs_revision"),
  };
}
