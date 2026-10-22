import { afterEach, describe, expect, it, vi } from "vitest";
import {
  ACTIONS,
  actionLabel,
  detailRows,
  formatTimestamp,
  listAuditLogs,
} from "./audit-logs";
import { currentPeriod } from "./reports";

// The Logs tab opens on the month in progress, counted in Asia/Manila (UTC+8).
describe("currentPeriod", () => {
  it.each([
    ["2026-05-15T04:00:00Z", "May", 2026],
    ["2026-05-31T15:59:00Z", "May", 2026], // 23:59 on 31 May in Manila
    ["2026-05-31T16:00:00Z", "June", 2026], // 00:00 on 1 June in Manila
    ["2025-12-31T16:00:00Z", "January", 2026], // 1 January in Manila
  ])("%s -> %s %i", (now, month, year) => {
    expect(currentPeriod(new Date(now))).toEqual({ month, year });
  });
});

describe("listAuditLogs", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("always asks for 50 per page and drops an empty action", async () => {
    const f = vi.fn().mockImplementation(() => Promise.resolve(new Response(JSON.stringify({ data: [], meta: {} }))));
    vi.stubGlobal("fetch", f);
    await listAuditLogs({ month: "May", year: 2026 });
    await listAuditLogs({ month: "May", year: 2026, action: "LOGIN_FAILED", page: 3 });
    const url = (i: number) => String(f.mock.calls[i][0]).replace(/^undefined/, "");
    expect(url(0)).toBe("/admin/audit-logs?month=May&year=2026&limit=50");
    expect(url(1)).toBe("/admin/audit-logs?month=May&year=2026&action=LOGIN_FAILED&page=3&limit=50");
  });
});

describe("actions", () => {
  it("labels all 21 audited actions, each once", () => {
    expect(ACTIONS).toHaveLength(21);
    expect(new Set(ACTIONS.map((a) => a.code)).size).toBe(21);
    expect(actionLabel("CHANGE_USER_ROLE")).toBe("Changed user role");
    expect(actionLabel("DELETE_ROLE")).toBe("Deleted role");
    expect(actionLabel("LOGIN_FAILED")).toBe("Failed login");
    expect(actionLabel("APPROVE_REPORT")).toBe("Approved report");
  });

  it("shows an action it does not know as its raw code", () => {
    expect(actionLabel("NEW_THING")).toBe("NEW_THING");
  });
});

describe("detailRows", () => {
  it.each([
    ["APPROVE_REPORT", { month: "September", year: 2026, count: 1 }, [["Period", "September 2026"]]],
    ["DOWNLOAD_BULK_EXCEL", { month: "September", year: 2026, count: 12 }, [["Period", "September 2026"], ["Reports", "12"]]],
    ["CHANGE_USER_STATUS", { from: "active", to: "inactive", sessions_revoked: 2 }, [["Status", "Active → Inactive"], ["Sessions ended", "2"]]],
    ["ADMIN_RESET_PASSWORD", { sessions_ended: 0 }, [["Sessions ended", "0"]]],
    ["LOGIN_FAILED", { reason: "wrong_password" }, [["Reason", "Wrong password"]]],
    ["LOGIN_FAILED", { reason: "unknown_account" }, [["Reason", "Unknown account"]]],
    ["LOGIN_FAILED", { reason: "deceased" }, [["Reason", "Account deceased"]]],
    ["DELETE_USER", { reason: "Left the ministry", sessions_revoked: 1 }, [["Reason", "Left the ministry"], ["Sessions ended", "1"]]],
    ["DELETE_USER", { reason: null, sessions_revoked: 0 }, [["Reason", "—"], ["Sessions ended", "0"]]],
    ["CREATE_USER", { role_id: 3 }, [["Role ID", "3"]]],
    ["CREATE_ROLE", { name: "Reviewer", permissions: ["reports:approve", "reports:read_any"] },
      [["Role", "Reviewer"], ["Permissions", "reports:approve, reports:read_any"]]],
    ["CREATE_ROLE", { name: "Empty", permissions: [] }, [["Role", "Empty"], ["Permissions", "—"]]],
    ["UPDATE_ROLE", { from: "Reviewer", to: "Senior reviewer", added: ["users:read"], removed: [] },
      [["Name", "Reviewer → Senior reviewer"], ["Permissions added", "users:read"], ["Permissions removed", "—"]]],
    ["UPDATE_ROLE", { added: [], removed: ["reports:approve"] }, [["Permissions added", "—"], ["Permissions removed", "reports:approve"]]],
    ["DELETE_ROLE", { name: "Reviewer" }, [["Role", "Reviewer"]]],
    ["CHANGE_USER_ROLE", { from: "user", to: "CGP Coordinator" }, [["Role", "user → CGP Coordinator"]]],
    ["LOGIN", {}, []],
  ] as const)("%s %j", (action, details, want) => {
    expect(detailRows(action, details)).toEqual(want);
  });

  it("never shows a key it has no label for on a known action", () => {
    expect(detailRows("LOGIN", { ip_address: "10.0.0.1", password: "x" })).toEqual([]);
  });

  it("lists the keys as they are for an action it does not know", () => {
    expect(detailRows("NEW_THING", { a: 1, b: "x" })).toEqual([["a", "1"], ["b", "x"]]);
  });
});

describe("formatTimestamp", () => {
  it("prints Manila time, with seconds and the zone when asked", () => {
    expect(formatTimestamp("2026-10-22T08:05:09Z")).toBe("Oct 22, 2026, 4:05 PM");
    expect(formatTimestamp("2026-10-22T08:05:09Z", true)).toBe("Oct 22, 2026, 4:05:09 PM (Asia/Manila)");
  });
});
