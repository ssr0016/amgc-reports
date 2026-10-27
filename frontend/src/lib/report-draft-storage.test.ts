import { beforeEach, describe, expect, it } from "vitest";
import { clearReportDraftStorage, newReportStorageKey, reportStorageKey } from "./report-draft-storage";

const report = { id: 12, created_by: 7, month: "April", year: 2026 };

beforeEach(() => localStorage.clear());

describe("report draft storage cleanup", () => {
  it("clears a report key and its owner's new-report key", () => {
    localStorage.setItem(reportStorageKey(report.id), "draft");
    localStorage.setItem(newReportStorageKey(7, report.month, report.year), "draft");

    clearReportDraftStorage(report, 7);

    expect(localStorage.getItem(reportStorageKey(report.id))).toBeNull();
    expect(localStorage.getItem(newReportStorageKey(7, report.month, report.year))).toBeNull();
  });

  it("does not clear another user's new-report key", () => {
    const key = newReportStorageKey(9, report.month, report.year);
    localStorage.setItem(key, "draft");

    clearReportDraftStorage(report, 9);

    expect(localStorage.getItem(key)).toBe("draft");
  });
});
