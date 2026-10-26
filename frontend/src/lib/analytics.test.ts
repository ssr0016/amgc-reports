import { afterEach, describe, expect, it, vi } from "vitest";
import { getAttendanceAnalytics } from "./analytics";

afterEach(() => vi.unstubAllGlobals());

describe("analytics", () => {
  it("requests attendance analytics with encoded filters", async () => {
    const body = { year: 2026, metric: "worship_service", status: "approved", period: "q2", trend: [], churches: [] };
    const fetchMock = vi.fn().mockResolvedValueOnce(new Response(JSON.stringify(body)));
    vi.stubGlobal("fetch", fetchMock);

    expect(await getAttendanceAnalytics(2026, "worship_service", "approved", "q2", true)).toEqual(body);
    expect(String(fetchMock.mock.calls[0][0]).replace(/^undefined/, "")).toBe(
      "/admin/analytics/attendance?year=2026&metric=worship_service&status=approved&period=q2&compare=true",
    );
  });
});
