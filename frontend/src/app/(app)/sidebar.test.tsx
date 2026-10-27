import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { Sidebar } from "./sidebar";

const h = vi.hoisted(() => ({ isAdmin: true, canAdmin: true, path: "/reports", tab: "", permissions: [] as string[], user: { role: "admin" } }));
const reports = vi.hoisted(() => ({ getCurrentReport: vi.fn() }));
vi.mock("next/navigation", () => ({ usePathname: () => h.path, useSearchParams: () => new URLSearchParams({ tab: h.tab }) }));
vi.mock("@/lib/session", () => ({ useSession: () => ({ ...h, can: (p: string) => h.isAdmin || h.permissions.includes(p) }) }));
vi.mock("@/lib/reports", () => ({ defaultPeriod: () => ({ month: "September", year: 2026 }), getCurrentReport: reports.getCurrentReport }));
const nav = () => within(screen.getByRole("navigation", { name: "Main" }));
const renderSidebar = () => render(<QueryClientProvider client={new QueryClient()}><Sidebar /></QueryClientProvider>);

describe("sidebar", () => {
  beforeEach(() => { Object.assign(h, { isAdmin: true, canAdmin: true, path: "/reports", tab: "", permissions: [], user: { role: "admin" } }); reports.getCurrentReport.mockResolvedValue(null); });

  it("names the reports link Community Reports", () => {
    renderSidebar();
    expect(nav().getByRole("link", { name: "Community Reports" })).toHaveAttribute("href", "/reports");
  });

  it("groups all admin destinations and keeps account links last", () => {
    renderSidebar();
    expect(nav().getAllByRole("link").map((a) => a.getAttribute("href"))).toEqual([
      "/reports", "/admin?tab=reports", "/admin?tab=analytics", "/admin?tab=tracker", "/admin?tab=users", "/admin?tab=roles", "/admin?tab=logs", "/account/profile", "/account/password",
    ]);
  });

  it("shows a custom role only its allowed admin destinations", () => {
    h.isAdmin = false;
    h.permissions = ["tracker:read", "roles:write"];
    renderSidebar();
    expect(nav().getAllByRole("link").map((a) => a.textContent)).toEqual(["Analytics", "Tracker", "Profile", "Change Password"]);
  });

  it("shows workers reports and settings without admin links", () => {
    Object.assign(h, { isAdmin: false, canAdmin: false, user: { role: "user" }, permissions: ["reports:read", "reports:write"] });
    renderSidebar();
    expect(nav().getAllByRole("link").map((a) => a.textContent)).toEqual(["Community Reports", "New Report", "Profile"]);
    expect(nav().queryByText("Admin")).not.toBeInTheDocument();
    expect(nav().queryByRole("link", { name: "Change Password" })).not.toBeInTheDocument();
  });

  it.each([
    ["/reports/new", "", "Community Reports"], ["/reports/5/edit", "", "Community Reports"],
    ["/admin", "", "Admin All Reports"], ["/admin", "logs", "Admin Logs"],
    ["/admin", "unknown", "Admin All Reports"], ["/admin/users/new", "", "Admin Users"],
    ["/admin/roles/3/edit", "", "Admin Roles"], ["/account/profile", "", "Profile"], ["/account/password", "", "Change Password"],
  ])("marks one current link on %s?tab=%s", (path, tab, name) => {
    Object.assign(h, { path, tab });
    renderSidebar();
    const current = nav().getAllByRole("link").filter((a) => a.getAttribute("aria-current") === "page");
    if (name) expect(current).toEqual([nav().getByRole("link", { name })]);
    else expect(current).toHaveLength(0);
  });

  it("uses the same forbidden-tab fallback as the admin panel", () => {
    Object.assign(h, { isAdmin: false, permissions: ["tracker:read"], path: "/admin", tab: "roles" });
    renderSidebar();
    expect(nav().getByRole("link", { name: "Admin Analytics" })).toHaveAttribute("aria-current", "page");
  });

  it.each([
    ["draft", "Continue Draft"],
    ["pending", "View Report"],
    ["approved", "View Report"],
    ["needs_revision", "Revise Report"],
  ])("labels the current report %s", async (status, label) => {
    Object.assign(h, { isAdmin: false, canAdmin: false, user: { role: "user" }, permissions: ["reports:read", "reports:write"] });
    reports.getCurrentReport.mockResolvedValue({ id: 12, month: "September", year: 2026, status });
    renderSidebar();
    expect(await screen.findByRole("link", { name: label })).toHaveAttribute("href", "/reports/new");
  });
});
