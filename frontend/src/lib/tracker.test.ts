import { afterEach, describe, expect, it, vi } from "vitest";
import { getTracker, trackerCounts, trackerLabel, type TrackerRow } from "./tracker";

afterEach(() => vi.unstubAllGlobals());

const row = (status: TrackerRow["status"]): TrackerRow =>
  ({ user_id: 1, name: "A", email: "a@x", church_name: "", user_status: "active", status }) as TrackerRow;

describe("tracker", () => {
  it("getTracker reads GET /admin/tracker for the period", async () => {
    const body = { month: "September", year: 2026, rows: [] };
    const f = vi.fn().mockResolvedValueOnce(new Response(JSON.stringify(body)));
    vi.stubGlobal("fetch", f);

    expect(await getTracker("September", 2026)).toEqual(body);
    expect(String(f.mock.calls[0][0]).replace(/^undefined/, "")).toBe("/admin/tracker?month=September&year=2026");
  });

  it("counts submitted as approved plus for review", () => {
    const rows = ["approved", "for_review", "for_review", "not_submitted"].map((s) => row(s as TrackerRow["status"]));
    expect(trackerCounts(rows)).toEqual({ expected: 4, submitted: 3, approved: 1, forReview: 2, notSubmitted: 1 });
  });

  it("counts no rows as all zeros", () => {
    expect(trackerCounts([])).toEqual({ expected: 0, submitted: 0, approved: 0, forReview: 0, notSubmitted: 0 });
  });

  it("labels the statuses and passes an unknown one through", () => {
    expect(trackerLabel("for_review")).toBe("For Review");
    expect(trackerLabel("mystery")).toBe("mystery");
  });
});
