import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import AppLayout from "./layout";

const h = vi.hoisted(() => ({ status: "authenticated" }));
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace: vi.fn() }), usePathname: () => "/reports", useSearchParams: () => new URLSearchParams() }));
vi.mock("@/lib/session", () => ({
  useSession: () => ({ status: h.status, user: { name: "Ana", email: "a@b.c", role: "user" }, isAdmin: false, canAdmin: false, can: () => false, signOut: vi.fn() }),
}));

describe("app layout", () => {
  // WCAG 2.4.1: a way past the header on every page.
  it("starts with a skip link to the main content", () => {
    render(<AppLayout>content</AppLayout>);
    const skip = screen.getByRole("link", { name: "Skip to main content" });
    expect(skip).toHaveAttribute("href", "#main-content");
    // first in the tab order, before the logo
    expect(screen.getAllByRole("link")[0]).toBe(skip);
  });

  it("gives a non-admin reports and account settings", () => {
    render(<AppLayout>content</AppLayout>);
    const nav = screen.getByRole("navigation", { name: "Main" });
    expect(within(nav).getAllByRole("link").map((a) => a.textContent)).toEqual(["My Reports", "Profile", "Change Password"]);
    expect(screen.queryByRole("link", { name: "Admin" })).not.toBeInTheDocument();
  });

  it("puts the sidebar after the header and before the page", () => {
    render(<AppLayout>content</AppLayout>);
    const header = screen.getByRole("banner");
    const nav = screen.getByRole("navigation", { name: "Main" });
    expect(header.compareDocumentPosition(nav) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(nav.compareDocumentPosition(screen.getByText("content")) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it("opens a modal drawer and returns focus after Escape", async () => {
    const user = userEvent.setup();
    render(<AppLayout>content</AppLayout>);
    const menu = screen.getByRole("button", { name: "Menu" });
    expect(menu).toHaveAttribute("aria-expanded", "false");
    await user.click(menu);
    expect(menu).toHaveAttribute("aria-expanded", "true");
    const drawer = screen.getByRole("dialog", { name: "AMGCReports navigation" });
    expect(menu).toHaveAttribute("aria-controls", drawer.id);
    expect(within(drawer).getByRole("navigation", { name: "Mobile" })).toBeInTheDocument();
    await user.keyboard("{Escape}");
    expect(menu).toHaveAttribute("aria-expanded", "false");
    expect(menu).toHaveFocus();
  });

  it("closes the menu when an item is chosen", async () => {
    const user = userEvent.setup();
    render(<AppLayout>content</AppLayout>);
    const menu = screen.getByRole("button", { name: "Menu" });
    await user.click(menu);
    await user.click(within(screen.getByRole("dialog")).getByRole("link", { name: "My Reports" }));
    expect(menu).toHaveAttribute("aria-expanded", "false");
  });

  it("shows no header, and so no skip link or sidebar, while the session loads", () => {
    h.status = "loading";
    render(<AppLayout>content</AppLayout>);
    expect(screen.queryByRole("link", { name: "Skip to main content" })).not.toBeInTheDocument();
    expect(screen.queryByRole("navigation", { name: "Main" })).not.toBeInTheDocument();
    expect(screen.getByRole("main")).toHaveTextContent("Loading…");
    h.status = "authenticated";
  });
});
