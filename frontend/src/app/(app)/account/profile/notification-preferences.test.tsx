import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, expect, it, vi } from "vitest";
import { NotificationPreferences } from "./notification-preferences";

const h = vi.hoisted(() => ({ update: vi.fn(), save: vi.fn(), success: vi.fn(), error: vi.fn() }));
vi.mock("@/lib/auth", () => ({ updateDailyDigest: h.save }));
vi.mock("@/lib/session", () => ({ useSession: () => ({ user: { user_id: 1, daily_digest: true }, updateUser: h.update }) }));
vi.mock("sonner", () => ({ toast: { success: h.success, error: h.error } }));

beforeEach(() => vi.clearAllMocks());

it("updates and reflects the saved daily digest preference", async () => {
  h.save.mockResolvedValue({ user_id: 1, daily_digest: false });
  render(<NotificationPreferences />);
  const toggle = screen.getByRole("switch", { name: "Daily digest" });
  expect(toggle).toBeChecked();
  await userEvent.click(toggle);
  await waitFor(() => expect(h.save).toHaveBeenCalledWith(false));
  expect(h.update).toHaveBeenCalledWith({ user_id: 1, daily_digest: false });
  expect(h.success).toHaveBeenCalled();
});

it("keeps the prior state and reports failures", async () => {
  h.save.mockRejectedValue(new Error("offline"));
  render(<NotificationPreferences />);
  const toggle = screen.getByRole("switch", { name: "Daily digest" });
  await userEvent.click(toggle);
  await waitFor(() => expect(h.error).toHaveBeenCalledWith("Could not update daily digest preference."));
  expect(toggle).toBeChecked();
});
