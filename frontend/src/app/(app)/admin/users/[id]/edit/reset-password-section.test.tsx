import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/api";
import { ResetPasswordSection } from "./reset-password-section";

const GENERATED = "Gen3rated-Passw0rd-xK";
const h = vi.hoisted(() => ({
  me: { user_id: 1 },
  resetUserPassword: vi.fn(),
  toast: { success: vi.fn(), error: vi.fn() },
}));
vi.mock("sonner", () => ({ toast: h.toast }));
vi.mock("@/lib/session", () => ({ useSession: () => ({ isAdmin: true, user: h.me }) }));
vi.mock("@/lib/users", () => ({ resetUserPassword: h.resetUserPassword }));
vi.mock("@/lib/password", () => ({ generatePassword: () => GENERATED }));

const target = { id: 7, name: "Ana Cruz" };
const PW = "N3w-Passw0rd-Xv91";

beforeEach(() => {
  vi.clearAllMocks();
  h.me = { user_id: 1 };
  h.resetUserPassword.mockResolvedValue(undefined);
});

function setup() {
  const user = userEvent.setup();
  render(<ResetPasswordSection user={target} />);
  return user;
}
const openButton = () => within(screen.getByRole("group", { name: "Reset password" })).getByRole("button", { name: "Reset password" });
async function openDialog(user: ReturnType<typeof userEvent.setup>) {
  await user.click(openButton());
  return within(await screen.findByRole("alertdialog"));
}
async function fillBoth(user: ReturnType<typeof userEvent.setup>, d: ReturnType<typeof within>, pw = PW, confirm = pw) {
  await user.type(d.getByLabelText("New password"), pw);
  if (confirm) await user.type(d.getByLabelText("Confirm password"), confirm);
}

describe("ResetPasswordSection", () => {
  it("has a labelled group with a Reset password button", () => {
    setup();
    expect(screen.getByRole("group", { name: "Reset password" })).toHaveTextContent("Sets a new password for Ana Cruz and signs them out everywhere.");
    expect(openButton()).toBeEnabled();
  });

  it("is off with the reason on your own account", () => {
    h.me = { user_id: 7 };
    setup();
    expect(openButton()).toBeDisabled();
    const note = screen.getByText("You cannot reset your own password. Use the Change Password page instead.");
    expect(openButton()).toHaveAttribute("aria-describedby", note.id);
  });

  it("opens a dialog for the user with the two fields, Generate and Copy", async () => {
    const d = await openDialog(setup());
    expect(screen.getByRole("alertdialog")).toHaveAccessibleName("Reset password for Ana Cruz");
    expect(d.getByLabelText("New password")).toHaveValue("");
    expect(d.getByLabelText("Confirm password")).toHaveValue("");
    expect(d.getByRole("button", { name: "Generate random password" })).toBeInTheDocument();
    expect(d.getByRole("button", { name: "Copy password" })).toBeInTheDocument();
    expect(d.getByRole("button", { name: "Cancel" })).toBeInTheDocument();
    expect(d.getByRole("button", { name: "Reset password" })).toBeInTheDocument();
  });

  it("Generate fills both fields with the same value and shows it", async () => {
    const user = setup();
    const d = await openDialog(user);
    await user.click(d.getByRole("button", { name: "Generate random password" }));
    expect(d.getByLabelText("New password")).toHaveValue(GENERATED);
    expect(d.getByLabelText("Confirm password")).toHaveValue(GENERATED);
    expect(d.getByLabelText("New password")).toHaveAttribute("type", "text");
    expect(d.getByLabelText("Confirm password")).toHaveAttribute("type", "text");
  });

  it("empty fields are required, the first gets focus and nothing is sent", async () => {
    const user = setup();
    const d = await openDialog(user);
    await user.click(d.getByRole("button", { name: "Reset password" }));
    expect(h.resetUserPassword).not.toHaveBeenCalled();
    expect(await d.findByText("New password is required")).toBeInTheDocument();
    expect(d.getByText("Confirm password is required")).toBeInTheDocument();
    expect(d.getByLabelText("New password")).toHaveAttribute("aria-invalid", "true");
    expect(d.getByLabelText("New password")).toHaveFocus();
  });

  it("a different confirmation says so, gets focus and sends nothing", async () => {
    const user = setup();
    const d = await openDialog(user);
    await fillBoth(user, d, PW, PW + "x");
    await user.click(d.getByRole("button", { name: "Reset password" }));
    expect(h.resetUserPassword).not.toHaveBeenCalled();
    const msg = await d.findByText("Confirm password does not match");
    expect(d.getByLabelText("Confirm password")).toHaveAttribute("aria-describedby", msg.id);
    expect(d.getByLabelText("Confirm password")).toHaveFocus();
    expect(d.getByLabelText("New password")).toHaveValue(PW);
  });

  it("a valid submit resets once, announces it and shows the password once in a done state", async () => {
    const user = setup();
    const d = await openDialog(user);
    await fillBoth(user, d);
    await user.click(d.getByRole("button", { name: "Reset password" }));

    await waitFor(() => expect(h.toast.success).toHaveBeenCalledWith("Password reset for Ana Cruz"));
    expect(h.resetUserPassword).toHaveBeenCalledTimes(1);
    expect(h.resetUserPassword).toHaveBeenCalledWith(7, PW, PW);
    const dialog = screen.getByRole("alertdialog");
    expect(dialog).toHaveTextContent("Password reset");
    expect(dialog).toHaveTextContent("Ana Cruz is signed out everywhere. This password will not be shown again.");
    expect(within(dialog).getByLabelText("Password")).toHaveValue(PW);
    expect(within(dialog).getByLabelText("Password")).toHaveAttribute("readonly");
    expect(within(dialog).queryByLabelText("New password")).not.toBeInTheDocument();
    expect(within(dialog).getByRole("button", { name: "Close" })).toBeInTheDocument();
  });

  it("a server 422 on each field shows on that field", async () => {
    h.resetUserPassword.mockRejectedValue(
      new ApiError(422, "VALIDATION_ERROR", "validation failed", {
        new_password: "is too weak; use a longer, more varied password",
        confirm_password: "does not match",
      }),
    );
    const user = setup();
    const d = await openDialog(user);
    await fillBoth(user, d);
    await user.click(d.getByRole("button", { name: "Reset password" }));
    expect(await d.findByText(/^New password is too weak/)).toBeInTheDocument();
    expect(d.getByText("Confirm password does not match")).toBeInTheDocument();
    expect(d.getByLabelText("New password")).toHaveFocus();
    expect(d.getByLabelText("New password")).toHaveValue(PW);
  });

  it.each([
    [403, "FORBIDDEN", "You cannot reset your own password. Use the Change Password page instead.", "You cannot reset your own password. Use the Change Password page instead."],
    [404, "NOT_FOUND", "user not found", "User not found."],
  ])("a %i shows its message in an alert and keeps what was typed", async (status, code, serverMessage, shown) => {
    h.resetUserPassword.mockRejectedValue(new ApiError(status, code, serverMessage));
    const user = setup();
    const d = await openDialog(user);
    await fillBoth(user, d);
    await user.click(d.getByRole("button", { name: "Reset password" }));
    expect(await d.findByRole("alert")).toHaveTextContent(shown);
    expect(d.getByLabelText("New password")).toHaveValue(PW);
    expect(d.getByLabelText("Confirm password")).toHaveValue(PW);
  });

  it("a 429 shows the wait and disables the button", async () => {
    h.resetUserPassword.mockRejectedValue(new ApiError(429, "RATE_LIMITED", "slow down", undefined, 30));
    const user = setup();
    const d = await openDialog(user);
    await fillBoth(user, d);
    await user.click(d.getByRole("button", { name: "Reset password" }));
    expect(await d.findByRole("alert")).toHaveTextContent("Too many attempts. Try again in 30s.");
    expect(d.getByRole("button", { name: "Reset password" })).toBeDisabled();
  });

  it("while the request is pending, Cancel and the submit button are off and Escape does not close", async () => {
    let finish!: () => void;
    h.resetUserPassword.mockReturnValue(new Promise<void>((resolve) => (finish = resolve)));
    const user = setup();
    const d = await openDialog(user);
    await fillBoth(user, d);
    await user.click(d.getByRole("button", { name: "Reset password" }));
    expect(await d.findByRole("button", { name: "Resetting…" })).toBeDisabled();
    expect(d.getByRole("button", { name: "Cancel" })).toBeDisabled();
    await user.keyboard("{Escape}");
    expect(screen.getByRole("alertdialog")).toBeInTheDocument();
    expect(h.resetUserPassword).toHaveBeenCalledTimes(1);
    await act(async () => finish());
  });

  it("Copy puts the typed password on the clipboard and announces it", async () => {
    const user = setup();
    const d = await openDialog(user);
    await user.type(d.getByLabelText("New password"), PW);
    await user.click(d.getByRole("button", { name: "Copy password" }));
    expect(await navigator.clipboard.readText()).toBe(PW);
    expect(h.toast.success).toHaveBeenCalledWith("Copied");
  });

  it("Copy in the done state copies the password that was set", async () => {
    const user = setup();
    const d = await openDialog(user);
    await user.click(d.getByRole("button", { name: "Generate random password" }));
    await user.click(d.getByRole("button", { name: "Reset password" }));
    await screen.findByText(/This password will not be shown again/);
    await user.click(within(screen.getByRole("alertdialog")).getByRole("button", { name: "Copy password" }));
    expect(await navigator.clipboard.readText()).toBe(GENERATED);
  });

  it("a refused clipboard says so and the password stays readable", async () => {
    const user = setup();
    vi.spyOn(navigator.clipboard, "writeText").mockRejectedValue(new Error("denied"));
    const d = await openDialog(user);
    await user.click(d.getByRole("button", { name: "Generate random password" }));
    await user.click(d.getByRole("button", { name: "Copy password" }));
    expect(await d.findByText("Could not copy. Select the password and copy it yourself.")).toBeInTheDocument();
    expect(h.toast.success).not.toHaveBeenCalledWith("Copied");
    expect(d.getByLabelText("New password")).toHaveValue(GENERATED);
  });

  it("Close in the done state clears everything, and reopening starts empty", async () => {
    const user = setup();
    const d = await openDialog(user);
    await fillBoth(user, d);
    await user.click(d.getByRole("button", { name: "Reset password" }));
    await screen.findByText(/This password will not be shown again/);
    await user.click(within(screen.getByRole("alertdialog")).getByRole("button", { name: "Close" }));
    await waitFor(() => expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument());

    const again = await openDialog(user);
    expect(again.getByLabelText("New password")).toHaveValue("");
    expect(again.getByLabelText("Confirm password")).toHaveValue("");
    expect(screen.queryByText(PW)).not.toBeInTheDocument();
    expect(document.body.innerHTML).not.toContain(PW);
  });

  it("Escape in the done state also clears the password", async () => {
    const user = setup();
    const d = await openDialog(user);
    await fillBoth(user, d);
    await user.click(d.getByRole("button", { name: "Reset password" }));
    await screen.findByText(/This password will not be shown again/);
    await user.keyboard("{Escape}");
    await waitFor(() => expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument());
    expect(document.body.innerHTML).not.toContain(PW);
  });

  it("Cancel closes the form and clears what was typed", async () => {
    const user = setup();
    const d = await openDialog(user);
    await fillBoth(user, d);
    await user.click(d.getByRole("button", { name: "Cancel" }));
    await waitFor(() => expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument());
    const again = await openDialog(user);
    expect(again.getByLabelText("New password")).toHaveValue("");
    expect(h.resetUserPassword).not.toHaveBeenCalled();
  });
});
