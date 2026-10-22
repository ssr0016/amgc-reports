import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/api";
import type { AuditLog } from "@/lib/audit-logs";
import type { Page } from "@/lib/reports";
import { AuditLogs } from "./audit-logs";

const h = vi.hoisted(() => ({ listAuditLogs: vi.fn() }));
// Keep the real labels and formatting; replace only the network call.
vi.mock("@/lib/audit-logs", async (orig) => ({
  ...(await orig<typeof import("@/lib/audit-logs")>()),
  listAuditLogs: h.listAuditLogs,
}));
// Pin "now" so the default month is known: 22 October 2026 in Manila.
vi.mock("@/lib/reports", async (orig) => {
  const real = await orig<typeof import("@/lib/reports")>();
  return { ...real, currentPeriod: () => real.currentPeriod(new Date("2026-10-22T08:00:00Z")) };
});

const log = (id: number, extra: Partial<AuditLog> = {}): AuditLog =>
  ({
    id, action: "APPROVE_REPORT", target_type: "Report", target_id: String(id), target_name: `Church ${id}`,
    performed_by_name: "Ana Admin", performed_by_role: "admin", details: { month: "September", year: 2026, count: 1 },
    created_at: "2026-10-22T08:05:09Z", ...extra,
  }) as AuditLog;

const page = (data: AuditLog[], extra: Partial<Page<AuditLog>["meta"]> = {}): Page<AuditLog> =>
  ({ data, meta: { page: 1, limit: 50, total: data.length, total_pages: 1, ...extra } }) as Page<AuditLog>;

function renderLogs() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const user = userEvent.setup();
  render(
    <QueryClientProvider client={client}>
      <AuditLogs />
    </QueryClientProvider>,
  );
  return { user };
}

const lastFilters = () => h.listAuditLogs.mock.calls.at(-1)![0];

beforeEach(() => {
  vi.clearAllMocks();
  h.listAuditLogs.mockResolvedValue(page([log(1), log(2)]));
});

describe("audit logs: filters", () => {
  it("starts on the current Manila month and all actions", async () => {
    renderLogs();
    await screen.findByText("Church 1");
    expect(screen.getByLabelText("Month")).toHaveValue("October");
    expect(screen.getByLabelText("Year")).toHaveValue("2026");
    expect(screen.getByLabelText("Action")).toHaveValue("");
    expect(lastFilters()).toEqual({ month: "October", year: 2026, action: undefined, page: 1 });
  });

  it("lists every action by its label, and the years from 2026", async () => {
    renderLogs();
    await screen.findByText("Church 1");
    const actions = within(screen.getByLabelText("Action")).getAllByRole("option").map((o) => o.textContent);
    expect(actions).toHaveLength(18);
    expect(actions[0]).toBe("All actions");
    expect(actions).toContain("Failed login");
    const years = within(screen.getByLabelText("Year")).getAllByRole("option").map((o) => o.textContent);
    expect(years[0]).toBe("2026");
  });

  it("goes back to page 1 when a filter changes", async () => {
    h.listAuditLogs.mockResolvedValue(page([log(1)], { total: 120, total_pages: 3 }));
    const { user } = renderLogs();
    await screen.findByText("Church 1");
    await user.click(screen.getByRole("button", { name: "Next" }));
    expect(lastFilters().page).toBe(2);

    await user.selectOptions(screen.getByLabelText("Action"), "LOGIN_FAILED");
    expect(lastFilters()).toEqual({ month: "October", year: 2026, action: "LOGIN_FAILED", page: 1 });
    await user.click(screen.getByRole("button", { name: "Next" }));
    await user.selectOptions(screen.getByLabelText("Month"), "September");
    expect(lastFilters()).toMatchObject({ month: "September", page: 1 });
  });
});

describe("audit logs: table", () => {
  it("has the five columns, in order, and announces the total", async () => {
    h.listAuditLogs.mockResolvedValue(page([log(1)], { total: 73, total_pages: 2 }));
    renderLogs();
    await screen.findByText("Church 1");
    expect(screen.getAllByRole("columnheader").map((c) => c.textContent)).toEqual([
      "Timestamp", "Actor", "Action", "Target", "Details",
    ]);
    expect(screen.getByText("73 total logs")).toHaveAttribute("role", "status");
    expect(screen.getByText("Page 1 of 2")).toBeInTheDocument();
  });

  it("shows a row's time in Manila, the stored actor, the action label and the target", async () => {
    renderLogs();
    const cells = within((await screen.findByText("Church 1")).closest("tr")!).getAllByRole("cell");
    expect(cells[0]).toHaveTextContent("Oct 22, 2026, 4:05 PM");
    expect(cells[1]).toHaveTextContent("Ana Admin");
    expect(cells[1]).toHaveTextContent("admin");
    expect(cells[2]).toHaveTextContent("Approved report");
    expect(cells[3]).toHaveTextContent("Report");
    expect(cells[3]).toHaveTextContent("Church 1");
  });

  it("shows a dash for a failed login with no actor and no target", async () => {
    h.listAuditLogs.mockResolvedValue(
      page([log(5, { action: "LOGIN_FAILED", performed_by_name: undefined, performed_by_role: undefined, target_id: undefined, target_name: undefined, details: { reason: "unknown_account" } })]),
    );
    renderLogs();
    const cells = within((await screen.findByRole("button", { name: /View details/ })).closest("tr")!).getAllByRole("cell");
    expect(cells[2]).toHaveTextContent("Failed login");
    expect(cells[1]).toHaveTextContent("—");
    expect(cells[3]).toHaveTextContent("—");
  });

  it("shows an action it does not know as its raw code", async () => {
    h.listAuditLogs.mockResolvedValue(page([log(6, { action: "NEW_THING", details: {} })]));
    renderLogs();
    expect(await screen.findByText("NEW_THING")).toBeInTheDocument();
  });
});

describe("audit logs: details", () => {
  it("expands and collapses a row, with labelled values and no raw JSON", async () => {
    h.listAuditLogs.mockResolvedValue(
      page([log(3, { action: "CHANGE_USER_STATUS", target_type: "User", target_name: "t@example.com", details: { from: "active", to: "inactive", sessions_revoked: 2 } })]),
    );
    const { user } = renderLogs();
    const button = await screen.findByRole("button", { name: /View details.*t@example.com/ });
    expect(button).toHaveAttribute("aria-expanded", "false");

    await user.click(button);
    expect(button).toHaveAttribute("aria-expanded", "true");
    expect(button).toHaveFocus();
    const panel = document.getElementById(button.getAttribute("aria-controls")!)!;
    expect(panel).toHaveTextContent("Active → Inactive");
    expect(panel).toHaveTextContent("Sessions ended");
    expect(panel).toHaveTextContent("Oct 22, 2026, 4:05:09 PM (Asia/Manila)");
    expect(panel).toHaveTextContent("ID 3");
    expect(panel.textContent).not.toContain("{");

    await user.click(button);
    expect(button).toHaveAttribute("aria-expanded", "false");
    expect(document.getElementById(button.getAttribute("aria-controls")!)).toBeNull();
  });

  it("says when there is nothing more to show", async () => {
    h.listAuditLogs.mockResolvedValue(page([log(4, { action: "LOGIN", details: {} })]));
    const { user } = renderLogs();
    await user.click(await screen.findByRole("button", { name: /View details/ }));
    expect(screen.getByText("No further details.")).toBeInTheDocument();
  });

  it("can keep several rows open at once", async () => {
    const { user } = renderLogs();
    const [a, b] = await screen.findAllByRole("button", { name: /View details/ });
    await user.click(a);
    await user.click(b);
    expect(a).toHaveAttribute("aria-expanded", "true");
    expect(b).toHaveAttribute("aria-expanded", "true");
  });
});

describe("audit logs: states", () => {
  it("shows loading, then the empty message", async () => {
    h.listAuditLogs.mockResolvedValue(page([]));
    renderLogs();
    expect(screen.getByText("Loading…")).toBeInTheDocument();
    expect(await screen.findByText("No logs match these filters.")).toBeInTheDocument();
  });

  it("shows the error with a retry that asks again", async () => {
    h.listAuditLogs.mockRejectedValueOnce(new ApiError(500, "internal", "Something went wrong"));
    const { user } = renderLogs();
    expect(await screen.findByRole("alert")).toHaveTextContent("Something went wrong");
    await user.click(screen.getByRole("button", { name: "Retry" }));
    expect(await screen.findByText("Church 1")).toBeInTheDocument();
  });

  it("tells a non-admin they are not allowed on a 403", async () => {
    h.listAuditLogs.mockRejectedValue(new ApiError(403, "forbidden", "forbidden"));
    renderLogs();
    expect(await screen.findByText("You are not allowed to view audit logs.")).toBeInTheDocument();
  });
});
