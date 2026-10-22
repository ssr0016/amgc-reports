import { api } from "./api";
import type { Tracker, TrackerRow } from "./api-schema";

export type { Tracker, TrackerRow };

export const getTracker = (month: string, year: number) =>
  api<Tracker>(`/admin/tracker?${new URLSearchParams({ month, year: String(year) })}`);

// The three statuses the server sends (D-A3), in the order of the Status filter.
export const TRACKER_STATUSES = [
  { code: "approved", label: "Approved" },
  { code: "for_review", label: "Pending" },
  { code: "not_submitted", label: "Not Submitted" },
] as const;

// A value this code does not know is shown as it came, not hidden.
export const trackerLabel = (status: string) =>
  TRACKER_STATUSES.find((s) => s.code === status)?.label ?? status;

export function trackerCounts(rows: readonly TrackerRow[]) {
  const n = (status: string) => rows.filter((r) => r.status === status).length;
  const approved = n("approved");
  const forReview = n("for_review");
  return { expected: rows.length, submitted: approved + forReview, approved, forReview, notSubmitted: n("not_submitted") };
}
