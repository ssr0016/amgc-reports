import { afterEach, describe, expect, it, vi } from "vitest";
import { change, getAttendanceAnalytics } from "./analytics";

afterEach(() => vi.unstubAllGlobals());

describe("analytics", () => {
  it("requests analytics with encoded filters and church ids", async () => {
    const body = { year: 2026 };
    const fetchMock = vi.fn().mockResolvedValueOnce(new Response(JSON.stringify(body)));
    vi.stubGlobal("fetch", fetchMock);

    expect(await getAttendanceAnalytics({ year: 2026, metric: "worship_service", status: "approved", period: "q2", compare: true, churches: [3, 7] })).toEqual(body);
    expect(String(fetchMock.mock.calls[0][0]).replace(/^undefined/, "")).toBe(
      "/admin/analytics/attendance?year=2026&metric=worship_service&status=approved&period=q2&compare=true&churches=3%2C7",
    );
  });

  it("has no change from 0 or from missing data", () => {
    expect(change(5, 0)).toBeNull();
    expect(change(null, 5)).toBeNull();
    expect(change(5, null)).toBeNull();
    expect(change(15, 10)).toBeCloseTo(0.5);
  });
});
