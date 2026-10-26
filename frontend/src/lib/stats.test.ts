import { afterEach, describe, expect, it, vi } from "vitest";
import { getStats } from "./stats";

afterEach(() => vi.unstubAllGlobals());

describe("stats", () => {
  it("getStats reads GET /admin/stats and returns the body as is", async () => {
    const body = { month: "September", year: 2026, reports_total: 3, reports_approved: 1, reports_pending: 2, users_total: 7, admins_total: 2, expected_reports: 5, no_report: 2, churches_total: 4 };
    const f = vi.fn().mockResolvedValueOnce(new Response(JSON.stringify(body)));
    vi.stubGlobal("fetch", f);

    expect(await getStats()).toEqual(body);
    expect(String(f.mock.calls[0][0]).replace(/^undefined/, "")).toBe("/admin/stats");
    expect(f.mock.calls[0][1]?.method ?? "GET").toBe("GET");
  });
});
