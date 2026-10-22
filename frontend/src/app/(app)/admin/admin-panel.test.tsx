import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useSyncExternalStore } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AdminPanel } from "./admin-panel";

// The mock router behaves like Next's: replace() changes the URL, and the components reading it re-render.
const h = vi.hoisted(() => {
  const subs = new Set<() => void>();
  const state = {
    isAdmin: true,
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
vi.mock("@/lib/session", () => ({ useSession: () => ({ isAdmin: h.isAdmin }) }));
vi.mock("./stats-cards", () => ({ StatsCards: () => <p>stats</p> }));
vi.mock("./admin-reports", () => ({ AdminReports: () => <p>admin reports</p> }));
vi.mock("./users/users-list", () => ({ UsersPanel: () => <p>users panel</p> }));
vi.mock("./audit-logs", () => ({ AuditLogs: () => <p>audit logs</p> }));
vi.mock("./tracker", () => ({ Tracker: () => <p>tracker</p> }));

describe("AdminPanel", () => {
  beforeEach(() => {
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

  it("shows an admin the Reports, Tracker, Users and Logs tabs, Reports first", () => {
    h.isAdmin = true;
    render(<AdminPanel />);
    expect(screen.getByRole("heading", { level: 1, name: "Admin" })).toBeInTheDocument();
    const tabs = screen.getAllByRole("tab");
    expect(tabs.map((t) => t.textContent)).toEqual(["Reports", "Tracker", "Users", "Logs"]);
    expect(tabs[0]).toHaveAttribute("aria-selected", "true");
    expect(screen.getByRole("tabpanel")).toHaveAccessibleName("Reports");
  });

  it("puts the stats cards above the tabs for an admin, and shows a non-admin none", () => {
    h.isAdmin = true;
    const { unmount } = render(<AdminPanel />);
    const stats = screen.getByText("stats");
    expect(stats.compareDocumentPosition(screen.getByRole("tablist")) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    unmount();

    h.isAdmin = false;
    render(<AdminPanel />);
    expect(screen.queryByText("stats")).not.toBeInTheDocument();
  });

  it("moves between tabs with the arrow keys, Home and End", async () => {
    h.isAdmin = true;
    const user = userEvent.setup();
    render(<AdminPanel />);
    const [reports, tracker, , logs] = screen.getAllByRole("tab");

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
});
