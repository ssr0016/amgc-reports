import { api, downloadFile } from "./api";
import type { AttendanceAnalytics } from "./api-schema";

export type { AttendanceAnalytics };
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

export function getAttendanceAnalytics(year: number, metric: string, status: AttendanceStatus, period: AnalyticsPeriod, compare: boolean) {
  const query = new URLSearchParams({ year: String(year), metric, status, period, compare: String(compare) });
  return api<AttendanceAnalytics>(`/admin/analytics/attendance?${query}`);
}

// The same analytics as a formatted .xlsx, built by the server.
export function exportAttendanceAnalytics(year: number, metric: string, status: AttendanceStatus, period: AnalyticsPeriod, compare: boolean) {
  const query = new URLSearchParams({ year: String(year), metric, status, period, compare: String(compare) });
  return downloadFile(`/admin/analytics/attendance/excel?${query}`, undefined, "GET");
}
