import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

const h = vi.hoisted(() => ({
  canApprove: true,
  list: vi.fn(),
  markRead: vi.fn(),
  push: vi.fn(),
}));

vi.mock("next/navigation", () => ({ useRouter: () => ({ push: h.push }) }));
vi.mock("@/lib/session", () => ({ useSession: () => ({ can: () => h.canApprove }) }));
vi.mock("@/lib/notifications", () => ({
  listUnreadNotifications: h.list,
  markNotificationRead: h.markRead,
}));

import { NotificationBell } from "./notification-bell";

function renderBell() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: 0 } } });
  return render(<QueryClientProvider client={client}><NotificationBell /></QueryClientProvider>);
}

describe("NotificationBell", () => {
  it("hides when the user cannot approve reports", () => {
    h.canApprove = false;
    renderBell();
    expect(screen.queryByRole("button", { name: /notifications/i })).not.toBeInTheDocument();
    h.canApprove = true;
  });

  it("shows unread dot and marks an item read before navigating", async () => {
    h.list.mockResolvedValue([{ id: 7, type: "report_submitted", message: "Report submitted by Ana", metadata: { report_id: 42 }, created_at: "2026-11-03T08:00:00Z" }]);
    h.markRead.mockResolvedValue(undefined);
    renderBell();

    const trigger = await screen.findByRole("button", { name: /notifications, 1 unread/i });
    expect(trigger.querySelector("[data-unread-dot]")).toBeInTheDocument();
    await userEvent.click(trigger);
    const item = await screen.findByRole("menuitem", { name: "Report submitted by Ana" });
    await userEvent.click(item);

    await waitFor(() => expect(h.markRead).toHaveBeenCalledWith(7, expect.anything()));
    expect(h.push).toHaveBeenCalledWith("/admin?tab=reports");
  });

  it("shows an empty state", async () => {
    h.list.mockResolvedValue([]);
    renderBell();
    await userEvent.click(await screen.findByRole("button", { name: /notifications, no unread/i }));
    expect(await screen.findByText("No unread notifications")).toBeInTheDocument();
  });

  it("announces a capped count and refills older notifications after marking one read", async () => {
    const notifications = Array.from({ length: 51 }, (_, index) => ({
      id: index + 1,
      type: "report_submitted",
      message: `Report ${index + 1}`,
      metadata: { report_id: index + 1 },
      created_at: "2026-11-03T08:00:00Z",
    }));
    h.list.mockResolvedValueOnce(notifications.slice(0, 50)).mockResolvedValue(notifications.slice(1));
    h.markRead.mockResolvedValue(undefined);
    renderBell();

    await userEvent.click(await screen.findByRole("button", { name: "Notifications, at least 50 unread" }));
    expect(screen.getAllByRole("menuitem")).toHaveLength(50);
    expect(screen.queryByRole("menuitem", { name: "Report 51" })).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("menuitem", { name: "Report 1" }));

    await userEvent.click(screen.getByRole("button", { name: "Notifications, at least 50 unread" }));
    expect(await screen.findByRole("menuitem", { name: "Report 51" })).toBeInTheDocument();
    expect(screen.queryByRole("menuitem", { name: "Report 1" })).not.toBeInTheDocument();
    expect(screen.getAllByRole("menuitem")).toHaveLength(50);
  });
});
