import { render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import AppLayout from "./layout";

vi.mock("next/navigation", () => ({ useRouter: () => ({ replace: vi.fn() }), usePathname: () => "/reports" }));
vi.mock("@/lib/session", () => ({
  useSession: () => ({ status: "authenticated", user: { name: "Ana", email: "a@b.c", role: "admin" }, isAdmin: true, signOut: vi.fn() }),
}));

describe("app header", () => {
  // WCAG 1.4.10: at 320 px the row (Menu, name, profile menu, Sign out) can be wider than the screen.
  // jsdom has no layout, so this checks that the rows may wrap; e2e/a11y.spec.ts measures the page.
  it("lets its rows wrap instead of overflowing a narrow screen", () => {
    render(<AppLayout>content</AppLayout>);
    const header = screen.getByRole("banner");
    expect(header.firstElementChild).toHaveClass("flex-wrap");
    expect(screen.getByRole("button", { name: "Sign out" }).parentElement).toHaveClass("flex-wrap");
  });

  it("holds only the app name, the profile menu and Sign out, with no page links", () => {
    render(<AppLayout>content</AppLayout>);
    const header = within(screen.getByRole("banner"));
    expect(header.getAllByRole("link").map((a) => a.textContent)).toEqual(["AMGCReports"]);
    expect(header.getByRole("button", { name: "Ana · admin" })).toBeInTheDocument();
    expect(header.getByRole("button", { name: "Sign out" })).toBeInTheDocument();
    for (const name of ["Admin", "Users", "Account"]) {
      expect(header.queryByRole("link", { name })).not.toBeInTheDocument();
    }
  });

  it("links an admin to the admin panel from the sidebar, with no separate Users link", () => {
    render(<AppLayout>content</AppLayout>);
    const nav = within(screen.getByRole("navigation", { name: "Main" }));
    expect(nav.getByRole("link", { name: "Admin" })).toHaveAttribute("href", "/admin");
    expect(nav.queryByRole("link", { name: "Users" })).not.toBeInTheDocument();
  });
});
