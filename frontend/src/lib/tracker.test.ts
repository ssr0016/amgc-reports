import { afterEach, describe, expect, it, vi } from "vitest";
import { getTracker, trackerCounts, trackerLabel, type TrackerRow } from "./tracker";

afterEach(() => vi.unstubAllGlobals());

const row = (submission_status: TrackerRow["submission_status"], review_status?: TrackerRow["review_status"]): TrackerRow =>
  ({ user_id: 1, name: "A", email: "a@x", church_name: "", user_status: "active", submission_status, review_status }) as TrackerRow;

describe("tracker", () => {
  it("getTracker reads GET /admin/tracker for the period", async () => {
    const body = { month: "September", year: 2026, rows: [] };
    const f = vi.fn().mockResolvedValueOnce(new Response(JSON.stringify(body)));
    vi.stubGlobal("fetch", f);

    expect(await getTracker("September", 2026)).toEqual(body);
    expect(String(f.mock.calls[0][0]).replace(/^undefined/, "")).toBe("/admin/tracker?month=September&year=2026");
  });

  it("counts submission and review statuses independently", () => {
    const rows = [
      row("submitted", "approved"),
      row("submitted", "pending"),
      row("submitted", "needs_revision"),
      row("no_report"),
    ];
    expect(trackerCounts(rows)).toEqual({ expected: 4, submitted: 3, noReport: 1, approved: 1, pending: 1, needsRevision: 1 });
  });

  it("counts no rows as all zeros", () => {
    expect(trackerCounts([])).toEqual({ expected: 0, submitted: 0, noReport: 0, approved: 0, pending: 0, needsRevision: 0 });
  });

  it("labels the statuses and passes an unknown one through", () => {
    expect(trackerLabel("needs_revision")).toBe("Needs Revision");
    expect(trackerLabel("mystery")).toBe("mystery");
  });
});
