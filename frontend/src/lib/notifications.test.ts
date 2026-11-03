import { describe, expect, it, vi } from "vitest";

const h = vi.hoisted(() => ({ api: vi.fn() }));
vi.mock("./api", () => ({ api: h.api }));

import { listUnreadNotifications, markNotificationRead } from "./notifications";

describe("notifications API", () => {
  it("lists unread notifications through the API", async () => {
    const rows = [{ id: 7, type: "report_submitted", message: "Report submitted", metadata: { report_id: 42 } }];
    h.api.mockResolvedValue(rows);

    await expect(listUnreadNotifications()).resolves.toEqual(rows);
    expect(h.api).toHaveBeenCalledWith("/notifications");
  });

  it("marks one notification read", async () => {
    h.api.mockResolvedValue(undefined);

    await markNotificationRead(7);

    expect(h.api).toHaveBeenCalledWith("/notifications/7/read", { method: "PATCH" });
  });
});
