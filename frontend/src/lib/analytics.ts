import { api, downloadFile } from "./api";
import type { AttendanceAnalytics, ChurchReportingInput, ChurchScope } from "./api-schema";

export type {
  AttendanceAnalytics,
  ChurchMetric,
  ChurchScope,
  ComplianceCell,
  ComplianceRow,
  MetricSummary,
  TrendPoint,
} from "./api-schema";
export type AttendanceStatus = "approved" | "all";
export type AnalyticsPeriod =
  | "full_year"
  | "q1" | "q2" | "q3" | "q4"
  | "january" | "february" | "march" | "april" | "may" | "june"
  | "july" | "august" | "september" | "october" | "november" | "december";

export const ANALYTICS_PERIODS: { value: AnalyticsPeriod; label: string }[] = [
  { value: "full_year", label: "Full Year" },
  { value: "q1", label: "Q1" },
  { value: "q2", label: "Q2" },
  { value: "q3", label: "Q3" },
  { value: "q4", label: "Q4" },
  { value: "january", label: "January" },
  { value: "february", label: "February" },
  { value: "march", label: "March" },
  { value: "april", label: "April" },
  { value: "may", label: "May" },
  { value: "june", label: "June" },
  { value: "july", label: "July" },
  { value: "august", label: "August" },
  { value: "september", label: "September" },
  { value: "october", label: "October" },
  { value: "november", label: "November" },
  { value: "december", label: "December" },
];

// One filter for the page and every export, so they always describe the same scope.
export type AnalyticsFilters = {
  year: number;
  metric: string;
  status: AttendanceStatus;
  period: AnalyticsPeriod;
  compare: boolean;
  churches: number[]; // empty means every church
};

export type CsvKind = "weekly_detail" | "compliance";

function analyticsQuery(f: AnalyticsFilters, extra: Record<string, string> = {}) {
  const query = new URLSearchParams({ ...extra, year: String(f.year), metric: f.metric, status: f.status, period: f.period, compare: String(f.compare) });
  if (f.churches.length > 0) query.set("churches", f.churches.join(","));
  return query;
}

export function getAttendanceAnalytics(f: AnalyticsFilters) {
  return api<AttendanceAnalytics>(`/admin/analytics/attendance?${analyticsQuery(f)}`);
}

// The same analytics as a formatted .xlsx, built by the server.
export function exportAttendanceAnalytics(f: AnalyticsFilters) {
  return downloadFile(`/admin/analytics/attendance/excel?${analyticsQuery(f)}`, undefined, "GET");
}

// One flat CSV table: the raw weekly detail or the church-by-month compliance.
export function exportAnalyticsCsv(f: AnalyticsFilters, kind: CsvKind) {
  return downloadFile(`/admin/analytics/csv?${analyticsQuery(f, { kind })}`, undefined, "GET");
}

export function setChurchReporting(id: number, input: ChurchReportingInput) {
  return api<ChurchScope>(`/admin/churches/${id}/reporting`, { method: "PUT", body: input });
}

// change is the relative change, or null when either side has no data or the
// previous is 0: the same rule as the server's.
export function change(current: number | null | undefined, previous: number | null | undefined): number | null {
  if (current == null || previous == null || previous === 0) return null;
  return (current - previous) / previous;
}

export const COMPLIANCE_LABELS: Record<string, string> = {
  not_required: "Not Required",
  not_reviewed: "Start Not Set",
  inactive: "Church Inactive",
  not_yet_open: "Not Yet Open",
  awaiting_submission: "Awaiting Submission",
  overdue_grace: "Overdue — Grace Period",
  not_filed: "Not Filed",
  approved: "Approved",
  pending: "Pending",
  needs_revision: "Needs Revision",
};

// monthLabel shows "2026-09" as "Sep 2026".
export function monthLabel(ym: string | undefined): string {
  if (!ym) return "";
  const [y, m] = ym.split("-").map(Number);
  return new Date(y, m - 1, 1).toLocaleDateString("en-US", { month: "short", year: "numeric" });
}

export const TIMELINESS_LABELS: Record<string, string> = {
  on_time: "on time",
  late: "late",
  after_cutoff: "after cutoff",
  before_open: "before month end",
};
