import { render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ADMIN_AREA } from "@/lib/permissions";
import { AdminPanel } from "./admin-panel";

const h = vi.hoisted(() => ({ isAdmin: true, permissions: [] as string[], tab: "" }));
vi.mock("next/navigation", () => ({ useSearchParams: () => new URLSearchParams({ tab: h.tab }) }));
vi.mock("@/lib/session", () => ({ useSession: () => {
  const can = (p: string) => h.isAdmin || h.permissions.includes(p);
  return { isAdmin: h.isAdmin, can, canAdmin: ADMIN_AREA.some(can) };
} }));
vi.mock("./stats-cards", () => ({ StatsCards: () => <p>stats</p> }));
vi.mock("./admin-reports", () => ({ AdminReports: () => <p>admin reports</p> }));
vi.mock("./users/users-list", () => ({ UsersPanel: () => <p>users panel</p> }));
vi.mock("./audit-logs", () => ({ AuditLogs: () => <p>audit logs</p> }));
vi.mock("./tracker", () => ({ Tracker: () => <p>tracker</p> }));
vi.mock("./analytics", () => ({ Analytics: () => <p>analytics</p> }));
vi.mock("./roles", () => ({ RolesPanel: () => <p>roles panel</p> }));

describe("AdminPanel", () => {
  beforeEach(() => { Object.assign(h, { isAdmin: true, permissions: [], tab: "" }); });

  it.each([
    ["", "Reports", "admin reports"], ["unknown", "Reports", "admin reports"],
    ["reports", "Reports", "admin reports"], ["tracker", "Tracker", "tracker"],
    ["analytics", "Analytics", "analytics"],
    ["users", "Users", "users panel"], ["roles", "Roles", "roles panel"], ["logs", "Logs", "audit logs"],
  ])("renders only the selected section for ?tab=%s", (tab, name, text) => {
    h.tab = tab;
    render(<AdminPanel />);
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent(`Admin / ${name}`);
    expect(within(screen.getByRole("region", { name })).getByText(text)).toBeInTheDocument();
    expect(screen.queryByRole("tablist")).not.toBeInTheDocument();
    expect(screen.queryByText("stats") !== null).toBe(name !== "Reports" && name !== "Tracker");
  });

  it.each([
    [["tracker:read"], "analytics", "Analytics"], [["users:list"], "logs", "Users"],
    [["audit:read"], "reports", "Logs"], [["reports:approve"], "roles", "Reports"], [["reports:export"], "", "Reports"],
  ])("preserves the first permitted section for %j", (permissions, tab, name) => {
    Object.assign(h, { isAdmin: false, permissions, tab });
    render(<AdminPanel />);
    expect(screen.getByRole("region", { name })).toBeInTheDocument();
    expect(screen.queryByText("roles panel")).not.toBeInTheDocument();
  });

  it("denies access when none of the admin permissions apply", () => {
    Object.assign(h, { isAdmin: false, permissions: ["reports:read", "roles:write"], tab: "roles" });
    render(<AdminPanel />);
    expect(screen.getByText("You are not allowed to view the admin panel.")).toBeInTheDocument();
    expect(screen.queryByText("roles panel")).not.toBeInTheDocument();
    expect(screen.queryByText("stats")).not.toBeInTheDocument();
  });
});
