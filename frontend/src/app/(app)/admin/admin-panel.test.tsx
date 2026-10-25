import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useSyncExternalStore } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ADMIN_AREA } from "@/lib/permissions";
import { AdminPanel } from "./admin-panel";

// The mock router behaves like Next's: replace() changes the URL, and the components reading it re-render.
const h = vi.hoisted(() => {
  const subs = new Set<() => void>();
  const state = {
    isAdmin: true,
    perms: [] as string[], // a non-admin's permissions; the admin holds every one
    tab: null as string | null,
    replace: (() => {}) as (url: string) => void,
    subscribe: (fn: () => void) => (subs.add(fn), () => subs.delete(fn)),
  };
  state.replace = (url) => {
    state.tab = new URL(url, "http://x").searchParams.get("tab");
    subs.forEach((fn) => fn());
  };
  return state;
});
const replace = vi.fn(h.replace);
vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace }),
  useSearchParams: () => {
    const tab = useSyncExternalStore(h.subscribe, () => h.tab);
    return new URLSearchParams(tab ? { tab } : {});
  },
}));
vi.mock("@/lib/session", () => ({
  useSession: () => {
    const can = (p: string) => h.isAdmin || h.perms.includes(p);
    return { isAdmin: h.isAdmin, can, canAdmin: ADMIN_AREA.some(can) };
  },
}));
vi.mock("./stats-cards", () => ({ StatsCards: () => <p>stats</p> }));
vi.mock("./admin-reports", () => ({ AdminReports: () => <p>admin reports</p> }));
vi.mock("./users/users-list", () => ({ UsersPanel: () => <p>users panel</p> }));
vi.mock("./audit-logs", () => ({ AuditLogs: () => <p>audit logs</p> }));
vi.mock("./tracker", () => ({ Tracker: () => <p>tracker</p> }));
vi.mock("./roles", () => ({ RolesPanel: () => <p>roles panel</p> }));

describe("AdminPanel", () => {
  beforeEach(() => {
    h.perms = [];
    h.tab = null;
    replace.mockClear();
  });

  it("writes the tab to the URL with replace, so a refresh keeps it and Back leaves the page", async () => {
    h.isAdmin = true;
    const user = userEvent.setup();
    render(<AdminPanel />);
    await user.click(screen.getByRole("tab", { name: "Users" }));
    expect(replace).toHaveBeenLastCalledWith("/admin?tab=users");
    expect(screen.getByRole("tab", { name: "Users" })).toHaveAttribute("aria-selected", "true");

    await user.click(screen.getByRole("tab", { name: "Reports" }));
    expect(replace).toHaveBeenLastCalledWith("/admin?tab=reports");
    expect(screen.getByRole("tab", { name: "Reports" })).toHaveAttribute("aria-selected", "true");
  });

  it("follows the URL when it changes from outside", () => {
    h.isAdmin = true;
    h.tab = "users";
    render(<AdminPanel />);
    expect(screen.getByRole("tab", { name: "Users" })).toHaveAttribute("aria-selected", "true");
  });

  it("shows an admin the Reports, Tracker, Users, Roles and Logs tabs, Reports first", () => {
    h.isAdmin = true;
    render(<AdminPanel />);
    expect(screen.getByRole("heading", { level: 1, name: "Admin" })).toBeInTheDocument();
    const tabs = screen.getAllByRole("tab");
    expect(tabs.map((t) => t.textContent)).toEqual(["Reports", "Tracker", "Users", "Roles", "Logs"]);
    expect(tabs[0]).toHaveAttribute("aria-selected", "true");
    expect(screen.getByRole("tabpanel")).toHaveAccessibleName("Reports");
  });

  it("keeps shared stats off the Reports tab and shows them on other admin tabs", () => {
    h.isAdmin = true;
    const { unmount } = render(<AdminPanel />);
    expect(screen.queryByText("stats")).not.toBeInTheDocument();
    h.tab = "tracker";
    unmount();
    const { unmount: unmountTracker } = render(<AdminPanel />);
    const stats = screen.getByText("stats");
    expect(stats.compareDocumentPosition(screen.getByRole("tablist")) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    unmountTracker();
    h.tab = null;

    h.isAdmin = false;
    render(<AdminPanel />);
    expect(screen.queryByText("stats")).not.toBeInTheDocument();
  });

  it("moves between tabs with the arrow keys, Home and End", async () => {
    h.isAdmin = true;
    const user = userEvent.setup();
    render(<AdminPanel />);
    const [reports, tracker, , , logs] = screen.getAllByRole("tab");

    await user.tab();
    expect(reports).toHaveFocus();
    await user.keyboard("{ArrowRight}");
    expect(tracker).toHaveFocus();
    expect(tracker).toHaveAttribute("aria-selected", "true");
    expect(screen.getByRole("tabpanel")).toHaveAccessibleName("Tracker");
    await user.keyboard("{ArrowLeft}");
    expect(reports).toHaveFocus();
    await user.keyboard("{End}");
    expect(logs).toHaveFocus();
    expect(screen.getByRole("tabpanel")).toHaveAccessibleName("Logs");
    await user.keyboard("{Home}");
    expect(reports).toHaveFocus();
  });

  it("shows only the active panel", async () => {
    h.isAdmin = true;
    const user = userEvent.setup();
    render(<AdminPanel />);
    expect(screen.getAllByRole("tabpanel")).toHaveLength(1);
    await user.click(screen.getByRole("tab", { name: "Users" }));
    expect(screen.getAllByRole("tabpanel")).toHaveLength(1);
    expect(screen.getByRole("tabpanel")).toHaveAccessibleName("Users");
  });

  it("tells a non-admin they are not allowed, with no tabs", () => {
    h.isAdmin = false;
    render(<AdminPanel />);
    expect(screen.getByText("You are not allowed to view the admin panel.")).toBeInTheDocument();
    expect(screen.queryByRole("tablist")).not.toBeInTheDocument();
  });

  it("shows the admin reports table in the Reports tab, and not in the Users tab", async () => {
    h.isAdmin = true;
    const user = userEvent.setup();
    render(<AdminPanel />);
    expect(within(screen.getByRole("tabpanel")).getByText("admin reports")).toBeInTheDocument();

    await user.click(screen.getByRole("tab", { name: "Users" }));
    expect(screen.queryByText("admin reports")).not.toBeInTheDocument();
    expect(within(screen.getByRole("tabpanel")).getByText("users panel")).toBeInTheDocument();
  });

  it("opens the Logs tab for ?tab=logs", () => {
    h.isAdmin = true;
    h.tab = "logs";
    render(<AdminPanel />);
    expect(screen.getByRole("tab", { name: "Logs" })).toHaveAttribute("aria-selected", "true");
    expect(within(screen.getByRole("tabpanel")).getByText("audit logs")).toBeInTheDocument();
    h.tab = null;
  });

  it("opens the Roles tab for ?tab=roles", () => {
    h.isAdmin = true;
    h.tab = "roles";
    render(<AdminPanel />);
    expect(screen.getByRole("tab", { name: "Roles" })).toHaveAttribute("aria-selected", "true");
    expect(screen.getByText("roles panel")).toBeInTheDocument();
    h.tab = null;
  });

  it("opens the Tracker tab for ?tab=tracker", () => {
    h.isAdmin = true;
    h.tab = "tracker";
    render(<AdminPanel />);
    expect(screen.getByRole("tab", { name: "Tracker" })).toHaveAttribute("aria-selected", "true");
    expect(within(screen.getByRole("tabpanel")).getByText("tracker")).toBeInTheDocument();
    h.tab = null;
  });

  it("opens the Users tab for ?tab=users, and Reports for anything else", () => {
    h.isAdmin = true;
    h.tab = "users";
    const { unmount } = render(<AdminPanel />);
    expect(screen.getByRole("tab", { name: "Users" })).toHaveAttribute("aria-selected", "true");
    expect(screen.getByText("users panel")).toBeInTheDocument();
    unmount();

    h.tab = "nonsense";
    render(<AdminPanel />);
    expect(screen.getByRole("tab", { name: "Reports" })).toHaveAttribute("aria-selected", "true");
    h.tab = null;
  });

  it("does not render the table for a non-admin", () => {
    h.isAdmin = false;
    render(<AdminPanel />);
    expect(screen.queryByText("admin reports")).not.toBeInTheDocument();
  });

  describe("a custom role (37b)", () => {
    const tabNames = () => screen.getAllByRole("tab").map((t) => t.textContent);

    it("sees every area its permissions allow, but never Roles", () => {
      h.isAdmin = false;
      h.perms = ["reports:read", "reports:approve", "reports:export", "tracker:read", "users:list", "audit:read", "roles:write", "users:write"];
      render(<AdminPanel />);
      expect(tabNames()).toEqual(["Reports", "Tracker", "Users", "Logs"]);
      expect(screen.queryByText("stats")).not.toBeInTheDocument();
    });

    it("with only tracker:read sees the Tracker tab and the cards, and lands on Tracker", () => {
      h.isAdmin = false;
      h.perms = ["tracker:read"];
      render(<AdminPanel />);
      expect(tabNames()).toEqual(["Tracker"]);
      expect(screen.getByRole("tab", { name: "Tracker" })).toHaveAttribute("aria-selected", "true");
      expect(screen.getByText("stats")).toBeInTheDocument();
      expect(screen.queryByText("admin reports")).not.toBeInTheDocument();
    });

    // The cards gate themselves on tracker:read (stats-cards.test.tsx).
    it.each([["reports:approve"], ["reports:export"]])("with only %s sees the Reports tab", (perm) => {
      h.isAdmin = false;
      h.perms = [perm];
      render(<AdminPanel />);
      expect(tabNames()).toEqual(["Reports"]);
    });

    it("lands on the first allowed tab when ?tab= names one it may not open", () => {
      h.isAdmin = false;
      h.perms = ["reports:approve", "tracker:read"];
      h.tab = "roles";
      const { unmount } = render(<AdminPanel />);
      expect(screen.getByRole("tab", { name: "Reports" })).toHaveAttribute("aria-selected", "true");
      expect(screen.queryByText("roles panel")).not.toBeInTheDocument();
      unmount();

      h.perms = ["tracker:read"];
      h.tab = "logs";
      render(<AdminPanel />);
      expect(screen.getByRole("tab", { name: "Tracker" })).toHaveAttribute("aria-selected", "true");
      expect(screen.queryByText("audit logs")).not.toBeInTheDocument();
    });

    it("is not allowed in with only a plain user's permissions", () => {
      h.isAdmin = false;
      h.perms = ["users:read", "reports:read", "reports:read_any", "reports:write", "reports:delete"];
      render(<AdminPanel />);
      expect(screen.getByText("You are not allowed to view the admin panel.")).toBeInTheDocument();
      expect(screen.queryByRole("tablist")).not.toBeInTheDocument();
    });
  });
});
