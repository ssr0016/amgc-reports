import { render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { Sidebar } from "./sidebar";

const h = vi.hoisted(() => ({ isAdmin: true, path: "/reports" }));
vi.mock("next/navigation", () => ({ usePathname: () => h.path }));
vi.mock("@/lib/session", () => ({ useSession: () => ({ isAdmin: h.isAdmin }) }));

const nav = () => screen.getByRole("navigation", { name: "Main" });
const current = () => within(nav()).queryAllByRole("link").filter((a) => a.getAttribute("aria-current") === "page");

describe("sidebar", () => {
  it("shows an admin Reports and Admin, in that order, and nothing else", () => {
    h.isAdmin = true;
    h.path = "/reports";
    render(<Sidebar />);
    const links = within(nav()).getAllByRole("link");
    expect(links.map((a) => [a.textContent, a.getAttribute("href")])).toEqual([
      ["Reports", "/reports"],
      ["Admin", "/admin"],
    ]);
  });

  it("shows anyone else only Reports, and never Tracker or Logs", () => {
    h.isAdmin = false;
    render(<Sidebar />);
    expect(within(nav()).getAllByRole("link").map((a) => a.textContent)).toEqual(["Reports"]);
    expect(screen.queryByText(/tracker|logs/i)).not.toBeInTheDocument();
  });

  it.each([
    ["/reports", "Reports"],
    ["/reports/new", "Reports"],
    ["/reports/5/edit", "Reports"],
    ["/admin", "Admin"],
    ["/admin/users/new", "Admin"],
    ["/admin/users/3/edit", "Admin"],
  ])("marks only the matching item on %s", (path, name) => {
    h.isAdmin = true;
    h.path = path;
    render(<Sidebar />);
    expect(current().map((a) => a.textContent)).toEqual([name]);
  });

  it("marks no item on the password page", () => {
    h.isAdmin = true;
    h.path = "/account/password";
    render(<Sidebar />);
    expect(current()).toEqual([]);
  });
});
