import { api } from "./api";

// Temporary hand-written types mirroring backend/internal/model/report.go.
// F3 replaces them with types generated from the Swagger spec.

// A missing week means "not reported"; 0 means reported zero.
export type MetricWeeks = Partial<Record<"week1" | "week2" | "week3" | "week4" | "week5", number>>;

export type ReportInput = {
  month: string;
  year?: number;
  worker: string;
  area_assignment: string;
  church_name: string;
  names: string;
  narrative_report: string;
  challenges: string;
  prayer_request: string;
  metrics: Record<string, MetricWeeks>;
};

export type Report = Omit<ReportInput, "year"> & {
  id: number;
  created_by: number;
  year: number;
  completed: boolean;
  completed_at?: string;
  completed_by?: number;
  created_at: string;
  updated_at: string;
};

export type ReportMetadata = Pick<Report, "id" | "completed" | "completed_at" | "completed_by">;

export type Page<T> = {
  data: T[];
  meta: { page: number; limit: number; total: number; total_pages: number };
};

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
