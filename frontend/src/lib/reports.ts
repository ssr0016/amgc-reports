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
