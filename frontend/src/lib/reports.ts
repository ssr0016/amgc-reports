import { api, downloadFile } from "./api";
import type { ExcelPreview, MetricWeeks, Page, Report, ReportInput, ReportMetadata } from "./api-schema";

export type { ExcelPreview, MetricWeeks, Page, Report, ReportInput, ReportMetadata };

export type ReportFilters = {
  month?: string;
  year?: number;
  worker?: string;
  area?: string;
  church?: string;
  page?: number;
  limit?: number;
};

export function listReports(filters: ReportFilters = {}): Promise<Page<Report>> {
  const qs = new URLSearchParams();
  for (const [key, value] of Object.entries(filters)) {
    if (value !== undefined && value !== "") qs.set(key, String(value));
  }
  const query = qs.toString();
  return api<Page<Report>>(`/reports${query ? `?${query}` : ""}`);
}

export const getReport = (id: number) => api<Report>(`/reports/${id}`);

export const createReport = (input: ReportInput) =>
  api<Report>("/reports", { method: "POST", body: input });

// PUT replaces the whole report, including its metric set.
export const updateReport = (id: number, input: ReportInput) =>
  api<Report>(`/reports/${id}`, { method: "PUT", body: input });

export const deleteReport = (id: number) => api<void>(`/reports/${id}`, { method: "DELETE" });

// Toggles between approved and not approved.
export const toggleComplete = (id: number) =>
  api<ReportMetadata>(`/reports/${id}/complete`, { method: "PATCH" });

// Excel exports (D-A6), admin only. The server writes the audit row for each download.
export const exportReport = (id: number) => downloadFile(`/admin/reports/${id}/excel`);

export const exportReports = (month: string, year: number) => downloadFile("/admin/reports/excel", { month, year });

// The workbook either export would build, as text; writes no audit row (D-A15).
export const previewExcel = (body: { report_id: number } | { month: string; year: number }) =>
  api<ExcelPreview>("/admin/reports/excel/preview", { method: "POST", body });

export const MONTHS = [
  "January",
  "February",
  "March",
  "April",
  "May",
  "June",
  "July",
  "August",
  "September",
  "October",
  "November",
  "December",
] as const;

// Reports are filed for the month that just ended, so that is the list default
// (January shows December of the previous year). The server counts months in
// Asia/Manila, so this does too, whatever the browser's time zone is.
export const defaultPeriod = (now = new Date()) => manilaMonth(now, -1);

// The month in progress in Asia/Manila: the Logs tab opens on it, so today's activity shows.
export const currentPeriod = (now = new Date()) => manilaMonth(now, 0);

function manilaMonth(now: Date, offset: number) {
  const parts = new Intl.DateTimeFormat("en-US", { timeZone: "Asia/Manila", year: "numeric", month: "numeric" })
    .formatToParts(now);
  const get = (type: string) => Number(parts.find((p) => p.type === type)!.value);
  const d = new Date(Date.UTC(get("year"), get("month") - 1 + offset, 1));
  return { month: MONTHS[d.getUTCMonth()], year: d.getUTCFullYear() };
}

// The system started in 2026; earlier years are not offered.
export const FIRST_YEAR = 2026;
