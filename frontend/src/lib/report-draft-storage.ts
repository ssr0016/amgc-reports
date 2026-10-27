type ReportStorageContext = { id: number; created_by: number; month: string; year: number };

export const reportStorageKey = (id: number) => `report:${id}`;
export const newReportStorageKey = (userId: number, month: string, year: number) => `report:${userId}:${month}:${year}`;

export function clearReportDraftStorage(report: ReportStorageContext, userId?: number) {
  if (typeof window === "undefined") return;
  localStorage.removeItem(reportStorageKey(report.id));
  if (report.created_by === userId && userId !== undefined) {
    localStorage.removeItem(newReportStorageKey(userId, report.month, report.year));
  }
}
