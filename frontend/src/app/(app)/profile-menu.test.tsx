import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { ProfileMenu } from "./profile-menu";

const h = vi.hoisted(() => ({ role: "admin" }));
vi.mock("@/lib/session", () => ({
  useSession: () => ({ user: { name: "Ana", email: "a@b.c", role: h.role }, isAdmin: h.role === "admin" }),
}));

describe("profile menu", () => {
  it("is one button showing the name and role, with the email as its title", () => {
    h.role = "admin";
    render(<ProfileMenu onSignOut={vi.fn()} />);
    const trigger = screen.getByRole("button", { name: "Ana · admin" });
    expect(trigger).toHaveAttribute("title", "a@b.c");
  });

  it("reads 'no role' for a user without one", () => {
    h.role = "";
    render(<ProfileMenu onSignOut={vi.fn()} />);
    expect(screen.getByRole("button", { name: "Ana · no role" })).toBeInTheDocument();
  });

  it("opens with profile, password, and sign out actions", async () => {
    h.role = "admin";
    const user = userEvent.setup();
    const onSignOut = vi.fn();
    render(<ProfileMenu onSignOut={onSignOut} />);
    expect(screen.queryByRole("menu")).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Ana · admin" }));
    const items = screen.getAllByRole("menuitem");
    expect(items).toHaveLength(3);
    expect(items[0]).toHaveAccessibleName("Profile");
    expect(items[0]).toHaveAttribute("href", "/account/profile");
    expect(items[1]).toHaveAccessibleName("Change Password");
    expect(items[1]).toHaveAttribute("href", "/account/password");
    expect(items[2]).toHaveAccessibleName("Sign Out");
    await user.click(items[2]);
    expect(onSignOut).toHaveBeenCalledOnce();
  });

  it("hides Change Password from non-admins", async () => {
    h.role = "user";
    const user = userEvent.setup();
    render(<ProfileMenu onSignOut={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Ana · user" }));
    expect(screen.getAllByRole("menuitem").map((i) => i.textContent)).toEqual(["Profile", "Sign Out"]);
  });

  it("opens from the keyboard, and Escape closes it and returns focus to the button", async () => {
    h.role = "admin";
    const user = userEvent.setup();
    render(<ProfileMenu onSignOut={vi.fn()} />);
    const trigger = screen.getByRole("button", { name: "Ana · admin" });
    await user.tab();
    expect(trigger).toHaveFocus();
    await user.keyboard("{Enter}");
    expect(screen.getByRole("menu")).toBeInTheDocument();
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("menu")).not.toBeInTheDocument();
    expect(trigger).toHaveFocus();
  });
});
