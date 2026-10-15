import { api } from "./api";
import type { MetricWeeks, Page, Report, ReportInput, ReportMetadata } from "./api-schema";

export type { MetricWeeks, Page, Report, ReportInput, ReportMetadata };

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
export function defaultPeriod(now = new Date()) {
  const parts = new Intl.DateTimeFormat("en-US", { timeZone: "Asia/Manila", year: "numeric", month: "numeric" })
    .formatToParts(now);
  const get = (type: string) => Number(parts.find((p) => p.type === type)!.value);
  const prev = new Date(Date.UTC(get("year"), get("month") - 2, 1));
  return { month: MONTHS[prev.getUTCMonth()], year: prev.getUTCFullYear() };
}

// The system started in 2026; earlier years are not offered.
export const FIRST_YEAR = 2026;
