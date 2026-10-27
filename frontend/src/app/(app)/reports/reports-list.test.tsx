import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/api";
import type { Page, Report } from "@/lib/reports";
import { Reports } from "./reports-list";

const h = vi.hoisted(() => ({
  push: vi.fn(),
  replace: vi.fn(),
  search: "",
  me: { user_id: 1, role: "user", perms: [] as string[] },
  listReports: vi.fn(),
  deleteReport: vi.fn(),
  getReport: vi.fn(),
  toggleComplete: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: h.push, replace: h.replace }),
  useSearchParams: () => new URLSearchParams(h.search),
}));
vi.mock("@/lib/session", () => ({
  useSession: () => ({ user: { user_id: h.me.user_id, role: h.me.role }, can: (p: string) => h.me.perms.includes(p) }),
}));
// Keep the real constants and defaultPeriod; replace only the calls that hit the network.
vi.mock("@/lib/reports", async (orig) => ({
  ...(await orig<typeof import("@/lib/reports")>()),
  listReports: h.listReports,
  deleteReport: h.deleteReport,
  getReport: h.getReport,
  toggleComplete: h.toggleComplete,
}));

const report = (id: number, by: number, extra: Partial<Report> = {}): Report =>
  ({
    id, owner_status: "active", created_by: by, month: "April", year: 2026, metrics: {}, completed: false,
    created_at: "", updated_at: "", worker: `Worker ${id}`, ...extra,
  }) as Report;

const page = (data: Report[], extra: Partial<Page<Report>["meta"]> = {}): Page<Report> =>
  ({ data, meta: { page: 1, limit: 20, total: data.length, total_pages: 1, ...extra } }) as Page<Report>;

function renderList(...perms: string[]) {
  h.me.perms = perms;
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const user = userEvent.setup();
  render(
    <QueryClientProvider client={client}>
      <Reports />
    </QueryClientProvider>,
  );
  return { user, client };
}

const row = (text: string) => screen.getByText(text).closest("tr")!;

beforeEach(() => {
  vi.clearAllMocks();
  h.search = "";
  h.me.user_id = 1;
  h.me.role = "user";
  h.listReports.mockResolvedValue(page([report(1, 1), report(2, 2)]));
});

describe("reports list: access", () => {
  it.each(["user", "admin"])("requests the community feed for %s without stats cards", async (role) => {
    h.me.role = role;
    renderList("reports:read", "reports:read_any");
    await screen.findByText("Worker 1");
    expect(h.listReports).toHaveBeenCalledWith(expect.not.objectContaining({ mine: true }));
    expect(screen.getByRole("heading", { name: "Reports", level: 1 })).toBeInTheDocument();
    expect(screen.queryByRole("region", { name: "Report statistics" })).not.toBeInTheDocument();
  });

  it("without reports:read it says so and never asks the API", () => {
    renderList();
    expect(screen.getByText("You are not allowed to view reports.")).toBeInTheDocument();
    expect(h.listReports).not.toHaveBeenCalled();
  });

  it("a 403 from the API reads the same", async () => {
    h.listReports.mockRejectedValue(new ApiError(403, "FORBIDDEN", "no"));
    renderList("reports:read");
    expect(await screen.findByText("You are not allowed to view reports.")).toBeInTheDocument();
  });

  it("shows other API errors, and an empty state", async () => {
    h.listReports.mockRejectedValue(new ApiError(500, "ERROR", "Something broke."));
    renderList("reports:read");
    expect(await screen.findByRole("alert")).toHaveTextContent("Something broke.");
  });

  it("says when nothing matches", async () => {
    h.listReports.mockResolvedValue(page([]));
    renderList("reports:read");
    expect(await screen.findByText("No reports found")).toBeInTheDocument();
  });
});

describe("reports list: columns", () => {
  it("reads Worker, Month, Year, Area, Church, Status, Actions, in that order", async () => {
    renderList("reports:read");
    await screen.findByText("Worker 1");
    expect(screen.getAllByRole("columnheader").map((c) => { const copy = c.cloneNode(true) as HTMLElement; copy.querySelectorAll("[aria-hidden=true]").forEach((node) => node.remove()); return copy.textContent; })).toEqual([
      "Worker", "Month", "Year", "Area", "Church", "Status", "Actions",
    ]);
    // "Actions" is a visible heading, not just a screen reader label
    expect(screen.getByRole("columnheader", { name: "Actions" }).querySelector(".sr-only")).toBeNull();
  });

  it("a row's cells follow the same order, with a dash for what is empty", async () => {
    h.listReports.mockResolvedValue(
      page([report(1, 1, { area_assignment: "Metro Manila", church_name: "AMGC Church" }), report(2, 2, { completed: true })]),
    );
    renderList("reports:read", "reports:write");
    await screen.findByText("Worker 1");
    const full = within(row("Worker 1")).getAllByRole("cell").slice(1);
    expect(full.slice(0, 6).map((c) => { const copy = c.cloneNode(true) as HTMLElement; copy.querySelectorAll("[aria-hidden=true]").forEach((node) => node.remove()); return copy.textContent; })).toEqual(["Worker 1", "April", "2026", "Metro Manila", "AMGC Church", "Pending"]);
    expect(within(full[6]).getByRole("link", { name: /^Edit / })).toBeInTheDocument();
    const bare = within(row("Worker 2")).getAllByRole("cell").slice(1);
    expect(bare.slice(0, 6).map((c) => { const copy = c.cloneNode(true) as HTMLElement; copy.querySelectorAll("[aria-hidden=true]").forEach((node) => node.remove()); return copy.textContent; })).toEqual(["Worker 2", "April", "2026", "—", "—", "Approved"]);
  });
});

describe("reports list: accessible names", () => {
  // Every row has the same actions; a screen reader user needs to know which report each is for (WCAG 2.4.6, 4.1.2).
  it("each row action says which report it acts on", async () => {
    renderList("reports:read", "reports:write", "reports:delete");
    await screen.findByText("Worker 1");
    const mine = within(row("Worker 1"));
    expect(mine.getByRole("link", { name: "Edit April 2026, Worker 1" })).toBeInTheDocument();
    expect(mine.getByRole("button", { name: "Delete April 2026, Worker 1" })).toBeInTheDocument();
  });

  it("a report without a worker is named by its month alone", async () => {
    h.listReports.mockResolvedValue(page([report(1, 1, { worker: undefined })]));
    renderList("reports:read");
    expect(await screen.findByRole("button", { name: "Preview April 2026" })).toBeInTheDocument();
  });

  it("the page's main landmark is the skip link target", async () => {
    renderList("reports:read");
    await screen.findByText("Worker 1");
    expect(screen.getByRole("main")).toHaveAttribute("id", "main-content");
    expect(screen.getByRole("main")).toHaveAttribute("tabindex", "-1");
  });
});

// WCAG 4.1.3: filters apply without a submit, so a screen reader user hears what changed.
describe("reports list: live regions", () => {
  it("loading and the empty state are status messages", async () => {
    h.listReports.mockReturnValue(new Promise(() => {}));
    renderList("reports:read");
    expect(await screen.findByRole("status", { name: "Loading table" })).toBeInTheDocument();
  });

  it("says when nothing matches, as a status", async () => {
    h.listReports.mockResolvedValue(page([]));
    renderList("reports:read");
    await waitFor(() => expect(screen.getByRole("status")).toHaveTextContent("No reports found"));
  });

  it("the page summary is announced when it changes", async () => {
    h.listReports.mockResolvedValue(page([report(1, 1)], { total_pages: 3, total: 41 }));
    renderList("reports:read");
    await waitFor(() => expect(screen.getByRole("status")).toHaveTextContent("Page 1 of 3 (41 reports)"));
  });
});

describe("reports list: gated buttons", () => {
  it("keeps bulk export and notifications out of the personal header", async () => {
    h.me.role = "admin";
    renderList("reports:read", "reports:export");
    await screen.findByText("Worker 1");
    expect(screen.queryByRole("button", { name: "Download Bulk" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Notifications/ })).not.toBeInTheDocument();
  });

  it.each(["editor", "admin", "manager", "custom", ""])("hides New Report for role %s even with write permissions", async (role) => {
    h.me.role = role;
    renderList("reports:read", "reports:write", "reports:write_any");
    await screen.findByText("Worker 1");
    expect(screen.queryByRole("link", { name: "New Report" })).not.toBeInTheDocument();
  });

  it("shows New Report for a user with reports:write", async () => {
    renderList("reports:read", "reports:write");
    await screen.findByText("Worker 1");
    expect(screen.getByRole("link", { name: "New Report" })).toHaveAttribute("href", "/reports/new");
  });

  it("New Report needs reports:write", async () => {
    renderList("reports:read");
    await screen.findByText("Worker 1");
    expect(screen.queryByRole("link", { name: "New Report" })).not.toBeInTheDocument();
  });

  it("a plain user can edit and delete only their own report", async () => {
    renderList("reports:read", "reports:read_any", "reports:write", "reports:delete");
    await screen.findByText("Worker 1");
    expect(within(row("Worker 1")).getByRole("link", { name: /^Edit / })).toBeInTheDocument();
    expect(within(row("Worker 1")).getByRole("button", { name: /^Delete / })).toBeInTheDocument();
    expect(within(row("Worker 2")).getByRole("button", { name: /^Preview / })).toBeInTheDocument();
    expect(within(row("Worker 2")).queryByRole("button", { name: /^Delete / })).not.toBeInTheDocument();
  });

  it("_any permissions reach other people's reports", async () => {
    renderList("reports:read", "reports:write", "reports:write_any", "reports:delete", "reports:delete_any");
    await screen.findByText("Worker 2");
    expect(within(row("Worker 2")).getByRole("link", { name: /^Edit / })).toBeInTheDocument();
    expect(within(row("Worker 2")).getByRole("button", { name: /^Delete / })).toBeInTheDocument();
  });

  it("without write or delete, even the owner only views", async () => {
    renderList("reports:read");
    await screen.findByText("Worker 1");
    expect(within(row("Worker 1")).getByRole("button", { name: /^Preview / })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /^Delete / })).not.toBeInTheDocument();
  });

  it("offers approval only with reports:approve", async () => {
    h.listReports.mockResolvedValue(page([report(1, 1, { completed: true }), report(2, 2)]));
    renderList("reports:read", "reports:approve");
    await screen.findByText("Worker 1");
    expect(within(row("Worker 1")).getByRole("button", { name: /^Unapprove / })).toBeInTheDocument();
    expect(within(row("Worker 2")).getByRole("button", { name: /^Approve / })).toBeInTheDocument();
  });

  it("requires notes before unapproving an approved report", async () => {
    h.listReports.mockResolvedValue(page([report(1, 1, { completed: true })]));
    h.toggleComplete.mockResolvedValue({ id: 1, completed: false });
    const { user } = renderList("reports:read", "reports:approve");
    await screen.findByText("Worker 1");

    await user.click(within(row("Worker 1")).getByRole("button", { name: /^Unapprove / }));
    const dialog = screen.getByRole("alertdialog");
    expect(within(dialog).getByRole("button", { name: "Unapprove" })).toBeDisabled();
    await user.type(within(dialog).getByRole("textbox", { name: "Admin Notes" }), "Needs updated totals");
    await user.click(within(dialog).getByRole("button", { name: "Unapprove" }));

    await waitFor(() => expect(h.toggleComplete).toHaveBeenCalledWith(1, "Needs updated totals"));
  });
});

describe("reports list: delete", () => {
  const mine = ["reports:read", "reports:write", "reports:delete"];

  it("asks first, then deletes and reloads", async () => {
    h.deleteReport.mockResolvedValue(undefined);
    const { user } = renderList(...mine);
    await screen.findByText("Worker 1");
    await user.click(within(row("Worker 1")).getByRole("button", { name: /^Delete / }));
    expect(await screen.findByText("Delete April 2026?")).toBeInTheDocument();
    expect(h.deleteReport).not.toHaveBeenCalled();

    await user.click(within(screen.getByRole("alertdialog")).getByRole("button", { name: "Delete" }));
    await waitFor(() => expect(h.deleteReport).toHaveBeenCalledWith(1));
    await waitFor(() => expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument());
    await waitFor(() => expect(h.listReports).toHaveBeenCalledTimes(2));
  });

  // WCAG 2.4.3: the dialog has no trigger element, so without help focus falls to the page body.
  it("Escape closes the dialog and gives focus back to the Delete button that opened it", async () => {
    const { user } = renderList(...mine);
    await screen.findByText("Worker 1");
    const open = within(row("Worker 1")).getByRole("button", { name: /^Delete / });
    await user.click(open);
    await screen.findByRole("alertdialog");
    await user.keyboard("{Escape}");
    await waitFor(() => expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument());
    expect(open).toHaveFocus();
  });

  it("after a delete the row is gone, so focus goes to the main content", async () => {
    h.deleteReport.mockResolvedValue(undefined);
    h.listReports.mockResolvedValueOnce(page([report(1, 1), report(2, 2)])).mockResolvedValue(page([report(2, 2)]));
    const { user } = renderList(...mine);
    await screen.findByText("Worker 1");
    await user.click(within(row("Worker 1")).getByRole("button", { name: /^Delete / }));
    await user.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: "Delete" }));
    await waitFor(() => expect(screen.queryByText("Worker 1")).not.toBeInTheDocument());
    await waitFor(() => expect(screen.getByRole("main")).toHaveFocus());
  });

  it("Cancel closes the dialog without deleting", async () => {
    const { user } = renderList(...mine);
    await screen.findByText("Worker 1");
    await user.click(within(row("Worker 1")).getByRole("button", { name: /^Delete / }));
    await user.click(await screen.findByRole("button", { name: "Cancel" }));
    await waitFor(() => expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument());
    expect(h.deleteReport).not.toHaveBeenCalled();
  });

  it("a failed delete keeps the dialog open with the reason", async () => {
    h.deleteReport.mockRejectedValue(new ApiError(500, "ERROR", "Server error."));
    const { user } = renderList(...mine);
    await screen.findByText("Worker 1");
    await user.click(within(row("Worker 1")).getByRole("button", { name: /^Delete / }));
    await user.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: "Delete" }));
    expect(await within(screen.getByRole("alertdialog")).findByRole("alert")).toHaveTextContent("Server error.");
  });

  it("a 404 counts as deleted: the dialog closes", async () => {
    h.deleteReport.mockRejectedValue(new ApiError(404, "NOT_FOUND", "gone"));
    const { user } = renderList(...mine);
    await screen.findByText("Worker 1");
    await user.click(within(row("Worker 1")).getByRole("button", { name: /^Delete / }));
    await user.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: "Delete" }));
    await waitFor(() => expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument());
  });

  it("deleting the last row of a later page steps back one page", async () => {
    h.search = "page=3";
    h.listReports.mockResolvedValue(page([report(1, 1)], { page: 3, total_pages: 3, total: 41 }));
    h.deleteReport.mockResolvedValue(undefined);
    const { user } = renderList(...mine);
    await screen.findByText("Worker 1");
    await user.click(within(row("Worker 1")).getByRole("button", { name: /^Delete / }));
    await user.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: "Delete" }));
    await waitFor(() => expect(h.push).toHaveBeenCalledWith(expect.stringContaining("page=2")));
  });
});

describe("reports list: filters and pagination", () => {
  it("starts on the previous month in Asia/Manila, this year", async () => {
    renderList("reports:read");
    await screen.findByText("Worker 1");
    const { defaultPeriod } = await import("@/lib/reports");
    const def = defaultPeriod();
    expect(h.listReports).toHaveBeenCalledWith(expect.objectContaining({ month: def.month, year: def.year }));
    expect(screen.getByLabelText("Month")).toHaveValue(def.month);
  });

  it("month= in the URL means any month; page, text filters pass through", async () => {
    h.search = "month=&year=2026&worker=Ana&page=2";
    renderList("reports:read");
    await screen.findByText("Worker 1");
    expect(h.listReports).toHaveBeenCalledWith({
      month: undefined, year: 2026, worker: "Ana", area: undefined, church: undefined, page: 2,
    });
    expect(screen.getByLabelText("Worker")).toHaveValue("Ana");
  });

  it("labels the no-month choice \"All months\", and it is selected for month=", async () => {
    h.search = "month=&year=2026";
    renderList("reports:read");
    await screen.findByText("Worker 1");
    expect(screen.getByRole("option", { name: "All months" })).toBeInTheDocument();
    expect(screen.queryByRole("option", { name: "Any" })).not.toBeInTheDocument();
    expect(screen.getByLabelText("Month")).toHaveDisplayValue("All months");
  });

  it("an out-of-range year falls back to the default", async () => {
    h.search = "year=1999";
    renderList("reports:read");
    await screen.findByText("Worker 1");
    const { defaultPeriod } = await import("@/lib/reports");
    expect(h.listReports).toHaveBeenCalledWith(expect.objectContaining({ year: defaultPeriod().year }));
  });

  it("choosing a month pushes it to the URL", async () => {
    const { user } = renderList("reports:read");
    await screen.findByText("Worker 1");
    await user.selectOptions(screen.getByLabelText("Month"), "March");
    expect(h.push).toHaveBeenCalledWith(expect.stringMatching(/^\/reports\?month=March&year=\d{4}$/));
  });

  it("typing waits for a pause before it changes the URL", async () => {
    renderList("reports:read");
    await screen.findByText("Worker 1");
    fireEvent.change(screen.getByLabelText("Worker"), { target: { value: "An" } });
    fireEvent.change(screen.getByLabelText("Worker"), { target: { value: "Ana" } });
    expect(h.replace).not.toHaveBeenCalled();
    await waitFor(() => expect(h.replace).toHaveBeenCalledTimes(1));
    expect(h.replace).toHaveBeenCalledWith(expect.stringContaining("worker=Ana"));
  });

  it("Clear appears once there is text and empties the filters", async () => {
    h.search = "worker=Ana";
    const { user } = renderList("reports:read");
    await screen.findByText("Worker 1");
    await user.click(screen.getByRole("button", { name: "Clear" }));
    expect(screen.getByLabelText("Worker")).toHaveValue("");
    expect(h.push).toHaveBeenCalledWith(expect.not.stringContaining("worker="));
  });

  it("no Clear button without text", async () => {
    renderList("reports:read");
    await screen.findByText("Worker 1");
    expect(screen.queryByRole("button", { name: "Clear" })).not.toBeInTheDocument();
  });

  it("paginates: Previous is off on page 1, Next goes to page 2", async () => {
    h.listReports.mockResolvedValue(page([report(1, 1)], { total_pages: 3, total: 41 }));
    const { user } = renderList("reports:read");
    expect(await screen.findByText("Page 1 of 3 (41 reports)")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Previous" })).toBeDisabled();
    await user.click(screen.getByRole("button", { name: "Next" }));
    expect(h.push).toHaveBeenCalledWith(expect.stringContaining("page=2"));
  });

  it("Next is off on the last page", async () => {
    h.search = "page=3";
    h.listReports.mockResolvedValue(page([report(1, 1)], { page: 3, total_pages: 3, total: 41 }));
    renderList("reports:read");
    await screen.findByText("Page 3 of 3 (41 reports)");
    expect(screen.getByRole("button", { name: "Next" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Previous" })).toBeEnabled();
  });
});

describe("report owner badges", () => {
  it.each([
    ["active", undefined, false, false],
    ["inactive", undefined, false, false],
    ["disciplinary", undefined, false, false],
    ["active", "2026-10-21T09:00:00Z", true, false],
    ["deceased", undefined, false, true],
    ["deceased", "2026-10-21T09:00:00Z", true, true],
  ])("owner %s deleted %s keeps the report visible", async (owner_status, owner_deleted_at, deleted, archived) => {
    h.listReports.mockResolvedValue(page([report(1, 2, { owner_status, owner_deleted_at })]));
    renderList("reports:read");
    await screen.findByText("Worker 1");
    const workerRow = within(row("Worker 1"));
    expect(workerRow.queryByText("Deleted user") !== null).toBe(deleted);
    expect(workerRow.queryByText("Archived") !== null).toBe(archived);
    expect(workerRow.getByText("Pending")).toBeInTheDocument();
  });
});


describe("reports list: metadata preview", () => {
  it("fetches the owner's report details for the shared preview", async () => {
    h.getReport.mockResolvedValue(report(1, 1, { status: "needs_revision", admin_notes: "Please add the missing totals." }));
    const { user } = renderList("reports:read");

    await user.click(await screen.findByRole("button", { name: "Preview April 2026, Worker 1" }));
    const dialog = await screen.findByRole("alertdialog", { name: "Report Preview" });

    expect(h.getReport).toHaveBeenCalledWith(1);
    expect(within(dialog).getByText("Needs Revision")).toBeInTheDocument();
    expect(within(dialog).getByText("Admin Notes")).toBeInTheDocument();
    expect(within(dialog).getByText("Please add the missing totals.")).toBeInTheDocument();
  });

  it("never fetches details for a reader and returns focus on close", async () => {
    h.listReports.mockResolvedValue(page([report(1, 2, { narrative_report: "Private narrative", metrics: { tithes: { week1: 12345 } } } as Partial<Report>)]));
    const { user } = renderList("reports:read", "reports:read_any");
    const trigger = await screen.findByRole("button", { name: "Preview April 2026, Worker 1" });
    await user.click(trigger);
    const dialog = screen.getByRole("alertdialog", { name: "Report Preview" });
    expect(within(dialog).getByText("Submission metadata")).toBeInTheDocument();
    expect(within(dialog).queryByText("Private narrative")).not.toBeInTheDocument();
    expect(within(dialog).queryByText("12345")).not.toBeInTheDocument();
    expect(h.getReport).not.toHaveBeenCalled();
    await user.click(within(dialog).getByRole("button", { name: "Close" }));
    await waitFor(() => expect(trigger).toHaveFocus());
  });
});
