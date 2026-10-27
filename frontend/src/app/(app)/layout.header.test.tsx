import { render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { QueryProvider } from "@/lib/query";
import AppLayout from "./layout";

vi.mock("next/navigation", () => ({ useRouter: () => ({ replace: vi.fn() }), usePathname: () => "/reports", useSearchParams: () => new URLSearchParams() }));
vi.mock("@/lib/session", () => ({
  useSession: () => ({ status: "authenticated", user: { name: "Ana", email: "a@b.c", role: "admin" }, isAdmin: true, canAdmin: true, can: () => true, signOut: vi.fn() }),
}));
const renderLayout = () => render(<QueryProvider><AppLayout>content</AppLayout></QueryProvider>);

describe("app header", () => {
  // WCAG 1.4.10: at 320 px the row (Menu, name, profile menu) can be wider than the screen.
  // jsdom has no layout, so this checks that the rows may wrap; e2e/a11y.spec.ts measures the page.
  it("lets its rows wrap instead of overflowing a narrow screen", () => {
    renderLayout();
    const header = screen.getByRole("banner");
    expect(header.firstElementChild).toHaveClass("flex-wrap");
    expect(screen.getByRole("button", { name: "Ana · admin" }).parentElement).toHaveClass("flex-wrap");
  });

  it("holds only the app name and profile menu, with no page links", () => {
    renderLayout();
    const header = within(screen.getByRole("banner"));
    expect(header.getAllByRole("link").map((a) => a.textContent)).toEqual(["AMGCReports"]);
    expect(header.getByRole("button", { name: "Ana · admin" })).toBeInTheDocument();
    for (const name of ["Admin", "Users", "Account"]) {
      expect(header.queryByRole("link", { name })).not.toBeInTheDocument();
    }
  });

  it("puts admin destinations in the sidebar", () => {
    renderLayout();
    const nav = within(screen.getByRole("navigation", { name: "Main" }));
    expect(nav.getByRole("link", { name: "Admin All Reports" })).toHaveAttribute("href", "/admin?tab=reports");
    expect(nav.getByRole("link", { name: "Admin Users" })).toHaveAttribute("href", "/admin?tab=users");
  });
});
