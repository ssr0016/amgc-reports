import { render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { Sidebar } from "./sidebar";

const h = vi.hoisted(() => ({ isAdmin: true, canAdmin: true, path: "/reports", tab: "", permissions: [] as string[] }));
vi.mock("next/navigation", () => ({ usePathname: () => h.path, useSearchParams: () => new URLSearchParams({ tab: h.tab }) }));
vi.mock("@/lib/session", () => ({ useSession: () => ({ ...h, can: (p: string) => h.isAdmin || h.permissions.includes(p) }) }));
const nav = () => within(screen.getByRole("navigation", { name: "Main" }));

describe("sidebar", () => {
  beforeEach(() => { Object.assign(h, { isAdmin: true, canAdmin: true, path: "/reports", tab: "", permissions: [] }); });

  it("names the reports link Community Reports", () => {
    render(<Sidebar />);
    expect(nav().getByRole("link", { name: "Community Reports" })).toHaveAttribute("href", "/reports");
  });

  it("groups all admin destinations and keeps account links last", () => {
    render(<Sidebar />);
    expect(nav().getAllByRole("link").map((a) => a.getAttribute("href"))).toEqual([
      "/reports", "/admin?tab=reports", "/admin?tab=analytics", "/admin?tab=tracker", "/admin?tab=users", "/admin?tab=roles", "/admin?tab=logs", "/account/profile", "/account/password",
    ]);
  });

  it("shows a custom role only its allowed admin destinations", () => {
    h.isAdmin = false;
    h.permissions = ["tracker:read", "roles:write"];
    render(<Sidebar />);
    expect(nav().getAllByRole("link").map((a) => a.textContent)).toEqual(["Analytics", "Tracker", "Profile", "Change Password"]);
  });

  it("shows workers reports and settings without admin links", () => {
    h.isAdmin = h.canAdmin = false;
    render(<Sidebar />);
    expect(nav().getAllByRole("link").map((a) => a.textContent)).toEqual(["Profile", "Change Password"]);
  });

  it.each([
    ["/reports/new", "", "Community Reports"], ["/reports/5/edit", "", "Community Reports"],
    ["/admin", "", "Admin All Reports"], ["/admin", "logs", "Admin Logs"],
    ["/admin", "unknown", "Admin All Reports"], ["/admin/users/new", "", "Admin Users"],
    ["/admin/roles/3/edit", "", "Admin Roles"], ["/account/profile", "", "Profile"], ["/account/password", "", "Change Password"],
  ])("marks one current link on %s?tab=%s", (path, tab, name) => {
    Object.assign(h, { path, tab });
    render(<Sidebar />);
    const current = nav().getAllByRole("link").filter((a) => a.getAttribute("aria-current") === "page");
    if (name) expect(current).toEqual([nav().getByRole("link", { name })]);
    else expect(current).toHaveLength(0);
  });

  it("uses the same forbidden-tab fallback as the admin panel", () => {
    Object.assign(h, { isAdmin: false, permissions: ["tracker:read"], path: "/admin", tab: "roles" });
    render(<Sidebar />);
    expect(nav().getByRole("link", { name: "Admin Analytics" })).toHaveAttribute("aria-current", "page");
  });
});
