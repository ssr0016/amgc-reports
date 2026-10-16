import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { AdminPanel } from "./admin-panel";

const h = vi.hoisted(() => ({ isAdmin: true }));
vi.mock("@/lib/session", () => ({ useSession: () => ({ isAdmin: h.isAdmin }) }));
vi.mock("./stats-cards", () => ({ StatsCards: () => <p>stats</p> }));
vi.mock("./admin-reports", () => ({ AdminReports: () => <p>admin reports</p> }));
vi.mock("./users/users-list", () => ({ UsersPanel: () => <p>users panel</p> }));

describe("AdminPanel", () => {
  it("shows an admin the Reports and Users tabs, Reports first", () => {
    h.isAdmin = true;
    render(<AdminPanel />);
    expect(screen.getByRole("heading", { level: 1, name: "Admin" })).toBeInTheDocument();
    const tabs = screen.getAllByRole("tab");
    expect(tabs.map((t) => t.textContent)).toEqual(["Reports", "Users"]);
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
    const [reports, users] = screen.getAllByRole("tab");

    await user.tab();
    expect(reports).toHaveFocus();
    await user.keyboard("{ArrowRight}");
    expect(users).toHaveFocus();
    expect(users).toHaveAttribute("aria-selected", "true");
    expect(screen.getByRole("tabpanel")).toHaveAccessibleName("Users");
    await user.keyboard("{ArrowLeft}");
    expect(reports).toHaveFocus();
    await user.keyboard("{End}");
    expect(users).toHaveFocus();
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

  it("does not render the table for a non-admin", () => {
    h.isAdmin = false;
    render(<AdminPanel />);
    expect(screen.queryByText("admin reports")).not.toBeInTheDocument();
  });
});
