import { api } from "./api";
import type { AttendanceAnalytics } from "./api-schema";

export type { AttendanceAnalytics };
export type AttendanceStatus = "approved" | "all";

export function getAttendanceAnalytics(year: number, metric: string, status: AttendanceStatus) {
  const query = new URLSearchParams({ year: String(year), metric, status });
  return api<AttendanceAnalytics>(`/admin/analytics/attendance?${query}`);
}
