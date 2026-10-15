import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { resetCsrf } from "./api";
import {
  createReport,
  defaultPeriod,
  deleteReport,
  getReport,
  listReports,
  toggleComplete,
  updateReport,
} from "./reports";

// The server counts months in Asia/Manila (UTC+8), D-F8.
describe("defaultPeriod", () => {
  it.each([
    ["2026-05-15T04:00:00Z", "April", 2026],
    ["2026-05-31T15:59:00Z", "April", 2026], // 23:59 on 31 May in Manila
    ["2026-05-31T16:00:00Z", "May", 2026], // 00:00 on 1 June in Manila
    ["2026-01-10T00:00:00Z", "December", 2025],
    ["2025-12-31T16:00:00Z", "December", 2025], // 1 January in Manila
    ["2026-03-01T00:00:00Z", "February", 2026],
  ])("%s -> %s %i", (now, month, year) => {
    expect(defaultPeriod(new Date(now))).toEqual({ month, year });
  });
});

describe("reports API", () => {
  const stub = (...bodies: unknown[]) => {
    const f = vi.fn();
    for (const b of bodies) f.mockResolvedValueOnce(new Response(JSON.stringify(b)));
    vi.stubGlobal("fetch", f);
    return f;
  };
  const at = (f: ReturnType<typeof vi.fn>, i: number) =>
    [String(f.mock.calls[i][0]).replace(/^undefined/, ""), (f.mock.calls[i][1] as RequestInit).method] as const;
  beforeEach(() => resetCsrf());
  afterEach(() => vi.unstubAllGlobals());

  it("listReports drops empty filters and keeps zero-like values that mean something", async () => {
    let f = stub({ data: [] });
    await listReports();
    expect(at(f, 0)[0]).toBe("/reports");
    f = stub({ data: [] });
    await listReports({ month: "May", year: 2026, worker: "", area: undefined, page: 2 });
    expect(at(f, 0)[0]).toBe("/reports?month=May&year=2026&page=2");
  });

  it("uses the right verb and path for get, create, update, delete and approve", async () => {
    const f = stub({}, { csrf_token: "t" }, {}, {}, {}, { status: 0 }, {});
    await getReport(3);
    await createReport({ month: "May" } as never);
    await updateReport(3, { month: "May" } as never);
    await toggleComplete(3);
    expect([0, 2, 3, 4].map((i) => at(f, i).join(" "))).toEqual([
      "/reports/3 GET",
      "/reports POST",
      "/reports/3 PUT",
      "/reports/3/complete PATCH",
    ]);
  });

  it("deleteReport resolves on 204", async () => {
    const f = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({ csrf_token: "t" })))
      .mockResolvedValueOnce(new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", f);
    await expect(deleteReport(9)).resolves.toBeUndefined();
    expect(at(f, 1).join(" ")).toBe("/reports/9 DELETE");
  });
});
