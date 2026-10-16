import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/api";
import { ChangePassword } from "./change-password";

const h = vi.hoisted(() => ({ changePassword: vi.fn(), toast: { success: vi.fn(), error: vi.fn() } }));

vi.mock("sonner", () => ({ toast: h.toast }));
vi.mock("@/lib/auth", () => ({ changePassword: h.changePassword }));

beforeEach(() => vi.resetAllMocks());

async function fill(current = "Old!Passw0rd", next = "New!Passw0rd#1") {
  const user = userEvent.setup();
  render(<ChangePassword />);
  if (current) await user.type(screen.getByLabelText("Current password"), current);
  if (next) await user.type(screen.getByLabelText("New password"), next);
  return user;
}
const submit = () => screen.getByRole("button", { name: /Change password|Changing/ });

describe("ChangePassword", () => {
  it("asks for both passwords before calling the API", async () => {
    const user = await fill("", "");
    await user.click(submit());
    expect(h.changePassword).not.toHaveBeenCalled();
    expect(screen.getByLabelText("Current password")).toHaveAttribute("aria-invalid", "true");
    expect(screen.getByLabelText("New password")).toHaveAttribute("aria-invalid", "true");
    expect(screen.getByLabelText("Current password")).toHaveFocus();
  });

  it("sends both passwords, confirms with a toast and clears the form", async () => {
    h.changePassword.mockResolvedValue(undefined);
    const user = await fill();
    await user.click(submit());
    await waitFor(() => expect(h.toast.success).toHaveBeenCalledWith("Password changed"));
    expect(h.changePassword).toHaveBeenCalledWith("Old!Passw0rd", "New!Passw0rd#1");
    expect(screen.getByLabelText("Current password")).toHaveValue("");
    expect(screen.getByLabelText("New password")).toHaveValue("");
  });

  it("shows a 422 on the field it names and keeps what was typed", async () => {
    h.changePassword.mockRejectedValue(new ApiError(422, "VALIDATION_ERROR", "validation failed", { current_password: "is incorrect" }));
    const user = await fill();
    await user.click(submit());
    expect(await screen.findByText("Current password is incorrect")).toBeInTheDocument();
    expect(screen.getByLabelText("Current password")).toHaveAttribute("aria-invalid", "true");
    expect(screen.getByLabelText("Current password")).toHaveFocus();
    expect(screen.getByLabelText("New password")).toHaveValue("New!Passw0rd#1");
  });

  it("shows a new-password 422 on the new password field", async () => {
    h.changePassword.mockRejectedValue(new ApiError(422, "VALIDATION_ERROR", "validation failed", { new_password: "is too weak; use a longer, more varied password" }));
    const user = await fill();
    await user.click(submit());
    expect(await screen.findByText(/^New password is too weak/)).toBeInTheDocument();
    expect(screen.getByLabelText("New password")).toHaveAttribute("aria-invalid", "true");
  });

  it("says the account is locked on a 403", async () => {
    h.changePassword.mockRejectedValue(new ApiError(403, "FORBIDDEN", "account is locked, try again later"));
    const user = await fill();
    await user.click(submit());
    expect(await screen.findByRole("alert")).toHaveTextContent("This account is locked. Try again later.");
  });

  it("shows the wait on a 429 and disables the button", async () => {
    h.changePassword.mockRejectedValue(new ApiError(429, "RATE_LIMITED", "slow down", undefined, 30));
    const user = await fill();
    await user.click(submit());
    expect(await screen.findByRole("alert")).toHaveTextContent("Too many attempts. Try again in 30s.");
    expect(submit()).toBeDisabled();
  });
});
