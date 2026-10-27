import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/api";
import type { Page, Report } from "@/lib/reports";
import { reportStorageKey } from "@/lib/report-draft-storage";
import { AdminReports } from "./admin-reports";

const h = vi.hoisted(() => ({
  perms: [] as string[],
  listReports: vi.fn(),
  deleteReport: vi.fn(),
  toggleComplete: vi.fn(),
  requestRevision: vi.fn(),
  exportReport: vi.fn(),
  exportReports: vi.fn(),
  previewExcel: vi.fn(),
  getReport: vi.fn(),
  saveFile: vi.fn(),
  toast: { success: vi.fn(), error: vi.fn() },
}));

vi.mock("sonner", () => ({ toast: h.toast }));
vi.mock("@/lib/session", () => ({ useSession: () => ({ user: { user_id: 1 }, can: (p: string) => h.perms.includes(p) }) }));
// Keep the real constants and defaultPeriod; replace only the calls that hit the network.
vi.mock("@/lib/reports", async (orig) => ({
  ...(await orig<typeof import("@/lib/reports")>()),
  listReports: h.listReports,
  deleteReport: h.deleteReport,
  toggleComplete: h.toggleComplete,
  requestRevision: h.requestRevision,
  exportReport: h.exportReport,
  exportReports: h.exportReports,
  previewExcel: h.previewExcel,
  getReport: h.getReport,
}));
vi.mock("@/lib/api", async (orig) => ({ ...(await orig<typeof import("@/lib/api")>()), saveFile: h.saveFile }));

const report = (id: number, extra: Partial<Report> = {}): Report =>
  ({
    id, owner_status: "active", created_by: 1, month: "September", year: 2026, metrics: {}, completed: false, created_at: "", updated_at: "",
    worker: `Worker ${id}`, area_assignment: `Area ${id}`, church_name: `Church ${id}`, ...extra,
  }) as Report;

const page = (data: Report[], extra: Partial<Page<Report>["meta"]> = {}): Page<Report> =>
  ({ data, meta: { page: 1, limit: 20, total: data.length, total_pages: 1, ...extra } }) as Page<Report>;

function renderReports(...perms: string[]) {
  h.perms = perms;
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const user = userEvent.setup();
  render(
    <QueryClientProvider client={client}>
      <AdminReports />
    </QueryClientProvider>,
  );
  return { user, client };
}

const row = (text: string) => screen.getByText(text).closest("tr")!;

beforeEach(() => {
  vi.clearAllMocks();
  localStorage.clear();
  h.listReports.mockResolvedValue(page([report(1), report(2)]));
  h.getReport.mockResolvedValue(report(1));
});

describe("admin reports: table", () => {
  it("has the report columns, in order", async () => {
    renderReports("reports:read");
    await screen.findByText("Worker 1");
    const heads = screen.getAllByRole("columnheader").map((c) => { const copy = c.cloneNode(true) as HTMLElement; copy.querySelectorAll("[aria-hidden=true]").forEach((node) => node.remove()); return copy.textContent; });
    expect(heads).toEqual(["Worker", "Month", "Year", "Area", "Church", "Status", "Actions"]);
  });

  it("shows a report's cells, and a dash for what is empty", async () => {
    h.listReports.mockResolvedValue(
      page([report(1), report(2, { worker: undefined, area_assignment: undefined, church_name: undefined })]),
    );
    renderReports("reports:read");
    await screen.findByText("Worker 1");
    const cells = within(row("Worker 1")).getAllByRole("cell").map((c) => { const copy = c.cloneNode(true) as HTMLElement; copy.querySelectorAll("[aria-hidden=true]").forEach((node) => node.remove()); return copy.textContent; });
    expect(cells.slice(0, 5)).toEqual(["Worker 1", "September", "2026", "Area 1", "Church 1"]);
    const empty = within(screen.getAllByRole("row")[2]).getAllByRole("cell").map((c) => { const copy = c.cloneNode(true) as HTMLElement; copy.querySelectorAll("[aria-hidden=true]").forEach((node) => node.remove()); return copy.textContent; });
    expect([empty[0], empty[3], empty[4]]).toEqual(["—", "—", "—"]);
  });

  it("says Approved or Pending", async () => {
    h.listReports.mockResolvedValue(page([report(1, { completed: true }), report(2)]));
    renderReports("reports:read");
    await screen.findByText("Worker 1");
    expect(within(row("Worker 1")).getByText("Approved")).toBeInTheDocument();
    expect(within(row("Worker 2")).getByText("Pending")).toBeInTheDocument();
  });

	it("shows Revised for a pending report with a revision timestamp", async () => {
		h.listReports.mockResolvedValue(page([report(1, { status: "pending", last_revised_at: "2026-10-26T00:00:00Z" })]));
		renderReports("reports:read");
		await screen.findByText("Worker 1");
		expect(within(row("Worker 1")).getByText("Revised")).toBeInTheDocument();
	});

  it("has an Excel control named for its row", async () => {
    renderReports("reports:read", "reports:export");
    await screen.findByText("Worker 1");
    const excel = within(row("Worker 1")).getByRole("button", { name: /^Excel / });
    expect(excel).toBeEnabled();
    expect(excel).toHaveAccessibleName("Excel September 2026, Worker 1");
  });

  it("offers edit and delete with the matching permissions", async () => {
    h.listReports.mockResolvedValue(page([report(1, { completed: true }), report(2)]));
    renderReports("reports:read", "reports:approve", "reports:write", "reports:write_any", "reports:delete", "reports:delete_any");
    await screen.findByText("Worker 1");
    expect(within(row("Worker 1")).getByRole("link", { name: /^Edit / })).toBeInTheDocument();
    expect(within(row("Worker 1")).getByRole("button", { name: /^Delete / })).toBeInTheDocument();
  });
});

describe("admin reports: header and states", () => {
  it("counts the reports in a status message", async () => {
    h.listReports.mockResolvedValue(page([report(1), report(2)], { total: 41, total_pages: 3 }));
    renderReports("reports:read");
    expect(await screen.findByText("41 total reports")).toBeInTheDocument();
    expect(screen.getByText("41 total reports").closest("[role=status]")).not.toBeNull();
  });

  it("reads '1 total reports' for one", async () => {
    h.listReports.mockResolvedValue(page([report(1)]));
    renderReports("reports:read");
    expect(await screen.findByText("1 total reports")).toBeInTheDocument();
  });

  it("loading is a status message", () => {
    h.listReports.mockReturnValue(new Promise(() => {}));
    renderReports("reports:read");
    expect(screen.getByRole("status", { name: "Loading table" })).toBeInTheDocument();
  });

  it("an empty period says so, as a status, and keeps the table frame", async () => {
    h.listReports.mockResolvedValue(page([]));
    renderReports("reports:read");
    expect(await screen.findByText("No reports match these filters")).toBeInTheDocument();
    expect(screen.getByRole("table")).toBeInTheDocument();
  });

  it("shows an API error as an alert", async () => {
    h.listReports.mockRejectedValue(new ApiError(500, "ERROR", "Server error."));
    renderReports("reports:read");
    expect(await screen.findByRole("alert")).toHaveTextContent("Server error.");
  });

  it("without reports:read it says so and never asks the API", () => {
    renderReports();
    expect(screen.getByText("You are not allowed to view reports.")).toBeInTheDocument();
    expect(h.listReports).not.toHaveBeenCalled();
  });

  it("a 403 from the API reads the same", async () => {
    h.listReports.mockRejectedValue(new ApiError(403, "FORBIDDEN", "no"));
    renderReports("reports:read");
    expect(await screen.findByText("You are not allowed to view reports.")).toBeInTheDocument();
  });
});

describe("admin reports: filters", () => {
  afterEach(() => vi.useRealTimers());

  it("debounces all text filters for 300 ms without a Search button", async () => {
    renderReports("reports:read");
    await screen.findByText("Worker 1");
    expect(screen.queryByRole("button", { name: "Search" })).not.toBeInTheDocument();
    vi.useFakeTimers();
    fireEvent.change(screen.getByLabelText("Worker"), { target: { value: "Ana" } });
    await act(() => vi.advanceTimersByTimeAsync(200));
    fireEvent.change(screen.getByLabelText("Church"), { target: { value: "Bethel" } });
    fireEvent.change(screen.getByLabelText("Area"), { target: { value: "North" } });
    await act(() => vi.advanceTimersByTimeAsync(299));
    expect(h.listReports).toHaveBeenCalledTimes(1);
    await act(() => vi.advanceTimersByTimeAsync(1));
    expect(h.listReports).toHaveBeenCalledTimes(2);
    expect(h.listReports).toHaveBeenLastCalledWith(expect.objectContaining({ worker: "Ana", church: "Bethel", area: "North", page: undefined }));
  });

  it("Clear empties all text fields and cancels a pending search", async () => {
    renderReports("reports:read");
    await screen.findByText("Worker 1");
    const month = (screen.getByLabelText("Month") as HTMLSelectElement).value;
    vi.useFakeTimers();
    for (const label of ["Worker", "Church", "Area"]) {
      fireEvent.change(screen.getByLabelText(label), { target: { value: "pending" } });
    }
    fireEvent.click(screen.getByRole("button", { name: "Clear" }));
    for (const label of ["Worker", "Church", "Area"]) expect(screen.getByLabelText(label)).toHaveValue("");
    expect(screen.queryByRole("button", { name: "Clear" })).not.toBeInTheDocument();
    expect(screen.getByLabelText("Month")).toHaveValue(month);
    await act(() => vi.advanceTimersByTimeAsync(300));
    expect(h.listReports).toHaveBeenCalledTimes(1);
  });

  it("starts on the previous month in Asia/Manila", async () => {
    renderReports("reports:read");
    await screen.findByText("Worker 1");
    const { defaultPeriod } = await import("@/lib/reports");
    const def = defaultPeriod();
    expect(h.listReports).toHaveBeenCalledWith(expect.objectContaining({ month: def.month, year: def.year }));
    expect(screen.getByLabelText("Month")).toHaveValue(def.month);
    expect(screen.getByLabelText("Year")).toHaveValue(String(def.year));
  });

  it("has period dropdowns and worker, church and area search", async () => {
    renderReports("reports:read");
    await screen.findByText("Worker 1");
    expect(screen.getAllByRole("combobox").map((c) => c.id)).toEqual(["admin-month", "admin-year"]);
    expect(screen.getAllByRole("textbox")).toHaveLength(3);
    expect(screen.queryByRole("button", { name: "Clear" })).not.toBeInTheDocument();
    expect(screen.getByLabelText("Worker")).toHaveAttribute("placeholder", "e.g. Juan Dela Cruz");
    expect(screen.getByLabelText("Church")).toHaveAttribute("placeholder", "e.g. AMGC Church");
    expect(screen.getByLabelText("Area")).toHaveAttribute("placeholder", "e.g. Metro Manila");
  });

  it("deletes a report after confirmation", async () => {
    h.deleteReport.mockResolvedValue(undefined);
    localStorage.setItem(reportStorageKey(1), "draft");
    const { user } = renderReports("reports:read", "reports:delete", "reports:delete_any");
    await screen.findByText("Worker 1");
    await user.click(within(row("Worker 1")).getByRole("button", { name: /^Delete / }));
    await user.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: "Delete" }));
    await waitFor(() => expect(h.deleteReport).toHaveBeenCalledWith(1));
    expect(localStorage.getItem(reportStorageKey(1))).toBeNull();
  });

  it("Month lists the twelve months and no Any, so one view is one month", async () => {
    renderReports("reports:read");
    await screen.findByText("Worker 1");
    const options = within(screen.getByLabelText("Month")).getAllByRole("option").map((o) => o.textContent);
    expect(options).toEqual(["January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"]);
  });

  // The period follows the calendar: in 2027 the admin can still go back to 2026.
  it("Year runs from 2026 to the current year, and an earlier year can be chosen", async () => {
    vi.useFakeTimers({ toFake: ["Date"], now: new Date("2027-03-10T04:00:00Z") });
    const { user } = renderReports("reports:read");
    await screen.findByText("Worker 1");
    expect(within(screen.getByLabelText("Year")).getAllByRole("option").map((o) => o.textContent)).toEqual(["2026", "2027"]);
    expect(h.listReports).toHaveBeenLastCalledWith(expect.objectContaining({ month: "February", year: 2027 })); // March 2027: the previous month

    await user.selectOptions(screen.getByLabelText("Year"), "2026");
    await user.selectOptions(screen.getByLabelText("Month"), "October");
    await waitFor(() => expect(h.listReports).toHaveBeenLastCalledWith(expect.objectContaining({ month: "October", year: 2026 })));
  });

  it("the previous month of January is December of last year", async () => {
    vi.useFakeTimers({ toFake: ["Date"], now: new Date("2027-01-15T04:00:00Z") });
    renderReports("reports:read");
    await screen.findByText("Worker 1");
    expect(h.listReports).toHaveBeenLastCalledWith(expect.objectContaining({ month: "December", year: 2026 }));
  });

  it("choosing a month or a year refetches with it", async () => {
    const { user } = renderReports("reports:read");
    await screen.findByText("Worker 1");
    await user.selectOptions(screen.getByLabelText("Month"), "March");
    await waitFor(() => expect(h.listReports).toHaveBeenLastCalledWith(expect.objectContaining({ month: "March" })));
    await user.selectOptions(screen.getByLabelText("Year"), "2026");
    await waitFor(() => expect(h.listReports).toHaveBeenLastCalledWith(expect.objectContaining({ month: "March", year: 2026 })));
  });
});

describe("admin reports: Download Bulk", () => {
  it("is a button in the header", async () => {
    renderReports("reports:read", "reports:export");
    await screen.findByText("Worker 1");
    const bulk = screen.getByRole("button", { name: "Download Bulk" });
    expect(bulk).toBeEnabled();
    expect(bulk.closest("header")).toBeInTheDocument();
  });

  // jsdom has no layout, so the right side is checked as the DOM order (last in the row) and the class that pushes it there.
  it("sits with the header actions", async () => {
    renderReports("reports:read", "reports:export");
    await screen.findByText("Worker 1");
    const bulk = screen.getByRole("button", { name: "Download Bulk" });
    expect(bulk.closest("header")).toContainElement(screen.getByRole("heading", { name: "All Reports" }));
  });

  it("is disabled when the period has no reports", async () => {
    h.listReports.mockResolvedValue(page([]));
    renderReports("reports:read", "reports:export");
    await screen.findByText("No reports match these filters");
    expect(screen.getByRole("button", { name: "Download Bulk" })).toBeDisabled();
  });
});

describe("admin reports: export gate (37b)", () => {
  it("without reports:export there is no Excel or Download Bulk button, and no export call", async () => {
    renderReports("reports:read", "reports:approve");
    await screen.findByText("Church 1");
    expect(screen.queryByRole("button", { name: /^Excel/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Download Bulk" })).not.toBeInTheDocument();
    expect(h.previewExcel).not.toHaveBeenCalled();
    expect(h.exportReport).not.toHaveBeenCalled();
  });
});

describe("admin reports: Excel preview and downloads", () => {
  const file = { blob: new Blob(["PK"]), filename: "Church Growth Program Report May 2025.xlsx" };
  const sheet = (name: string) => ({ name, rows: [[`${name} A1`, "", "", "", "", "", ""]] });

  beforeEach(() => {
    h.previewExcel.mockResolvedValue({ count: 2, sheets: [sheet("Church 1"), sheet("Church 2")] });
  });

  it("Download Bulk opens the month's preview and downloads nothing yet", async () => {
    const { user } = renderReports("reports:read", "reports:export");
    await screen.findByText("Worker 1");
    await user.selectOptions(screen.getByLabelText("Month"), "May");
    await user.selectOptions(screen.getByLabelText("Year"), "2026");
    await user.click(screen.getByRole("button", { name: "Download Bulk" }));

    const dialog = await screen.findByRole("alertdialog", { name: "Excel preview: May 2026" });
    expect(h.previewExcel).toHaveBeenCalledWith({ month: "May", year: 2026 });
    expect(await within(dialog).findByText("Church 1 A1")).toBeInTheDocument();
    expect(h.exportReports).not.toHaveBeenCalled();
  });

  it("Download exports once, is busy until the file arrives, then closes", async () => {
    let resolve!: (v: typeof file) => void;
    h.exportReports.mockReturnValue(new Promise((r) => (resolve = r)));
    const { user } = renderReports("reports:read", "reports:export");
    await screen.findByText("Worker 1");
    await user.click(screen.getByRole("button", { name: "Download Bulk" }));
    const dialog = await screen.findByRole("alertdialog");
    await within(dialog).findByText("Church 1 A1");
    await user.click(within(dialog).getByRole("button", { name: "Download" }));

    expect(h.exportReports).toHaveBeenCalledTimes(1);
    const busy = within(dialog).getByRole("button", { name: "Downloading…" });
    expect(busy).toBeDisabled();
    expect(busy).toHaveAttribute("aria-busy", "true");

    resolve(file);
    await waitFor(() => expect(h.saveFile).toHaveBeenCalledWith(file));
    await waitFor(() => expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument());
    expect(screen.getByRole("button", { name: "Download Bulk" })).toBeEnabled();
    expect(h.toast.error).not.toHaveBeenCalled();
    expect(h.toast.success).toHaveBeenCalledWith(`Downloaded ${file.filename}`);
  });

  it("a row's Excel button previews that report; Cancel downloads nothing", async () => {
    h.previewExcel.mockResolvedValue({ count: 1, sheets: [sheet("Church 1")] });
    const { user } = renderReports("reports:read", "reports:export");
    await screen.findByText("Worker 1");
    await user.click(within(row("Worker 1")).getByRole("button", { name: /^Excel / }));

    const dialog = await screen.findByRole("alertdialog", { name: "Excel preview: Church 1" });
    expect(h.previewExcel).toHaveBeenCalledWith({ report_id: 1 });
    await within(dialog).findByText("Church 1 A1");
    await user.click(within(dialog).getByRole("button", { name: "Cancel" }));

    await waitFor(() => expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument());
    expect(h.exportReport).not.toHaveBeenCalled();
    expect(h.saveFile).not.toHaveBeenCalled();
  });

  it("a failed download shows the server's message and keeps the preview open", async () => {
    h.exportReport.mockRejectedValue(new ApiError(404, "NOT_FOUND", "Report not found."));
    const { user } = renderReports("reports:read", "reports:export");
    await screen.findByText("Worker 1");
    await user.click(within(row("Worker 2")).getByRole("button", { name: /^Excel / }));
    const dialog = await screen.findByRole("alertdialog");
    await within(dialog).findByText("Church 1 A1");
    await user.click(within(dialog).getByRole("button", { name: "Download" }));

    await waitFor(() => expect(h.toast.error).toHaveBeenCalledWith("Report not found."));
    expect(h.exportReport).toHaveBeenCalledWith(2);
    expect(h.saveFile).not.toHaveBeenCalled();
    expect(within(dialog).getByRole("button", { name: "Download" })).toBeEnabled();
  });
});

describe("admin reports: pages", () => {
  it("disables page buttons for a single page", async () => {
    renderReports("reports:read");
    await screen.findByText("Worker 1");
    expect(screen.getByRole("button", { name: "Next" })).toBeDisabled();
  });

  it("Next and Previous move the page, and a new filter goes back to page 1", async () => {
    h.listReports.mockImplementation((f: { page?: number }) =>
      Promise.resolve(page([report(1)], { page: f.page ?? 1, total_pages: 3, total: 41 })),
    );
    const { user } = renderReports("reports:read");
    expect(await screen.findByText("Page 1 of 3")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Previous" })).toBeDisabled();

    await user.click(screen.getByRole("button", { name: "Next" }));
    expect(await screen.findByText("Page 2 of 3")).toBeInTheDocument();
    expect(h.listReports).toHaveBeenLastCalledWith(expect.objectContaining({ page: 2 }));

    await user.click(screen.getByRole("button", { name: "Previous" }));
    await screen.findByText("Page 1 of 3");

    await user.click(screen.getByRole("button", { name: "Next" }));
    await screen.findByText("Page 2 of 3");
    await user.selectOptions(screen.getByLabelText("Month"), "March");
    await screen.findByText("Page 1 of 3");
    expect(h.listReports).toHaveBeenLastCalledWith(expect.not.objectContaining({ page: 2 }));
  });
});

describe("admin reports: approve", () => {
  const approve = (name: string) => within(row(name)).getByRole("button", { name: /^Approve / });

  const unapprove = (name: string) => within(row(name)).getByRole("button", { name: /^Unapprove / });

  it("Request Revision moves a pending row into the revision queue", async () => {
    h.requestRevision.mockResolvedValue({ id: 1, completed: false, status: "needs_revision" });
    const { user } = renderReports("reports:read", "reports:approve");
    await screen.findByText("Worker 1");
    await user.click(within(row("Worker 1")).getByRole("button", { name: /^Request Revision / }));
    await user.type(screen.getByRole("textbox", { name: /reason/i }), "Missing totals");
    await user.click(screen.getByRole("button", { name: /Request Revision/ }));
    await waitFor(() => expect(h.requestRevision).toHaveBeenCalledWith(1, "Missing totals"));
    expect(h.toast.success).toHaveBeenCalledWith("Revision requested: September 2026");
  });

  it("a Pending row offers Approve and a Approved row offers Unapprove, each named for its row", async () => {
    h.listReports.mockResolvedValue(page([report(1, { completed: true }), report(2)]));
    renderReports("reports:read", "reports:approve");
    await screen.findByText("Worker 1");
    expect(unapprove("Worker 1")).toHaveAccessibleName("Unapprove September 2026, Worker 1");
    expect(within(row("Worker 1")).queryByRole("button", { name: /^Approve / })).not.toBeInTheDocument();
    expect(approve("Worker 2")).toHaveAccessibleName("Approve September 2026, Worker 2");
    expect(within(row("Worker 2")).queryByRole("button", { name: /Unapprove/ })).not.toBeInTheDocument();
  });

  it("bulk approves only selected non-approved reports and reports skipped rows", async () => {
    h.listReports.mockResolvedValue(page([report(1, { completed: true }), report(2)]));
    h.toggleComplete.mockResolvedValue({ id: 2, completed: true });
    const { user } = renderReports("reports:read", "reports:approve");
    await screen.findByText("Worker 1");

    await user.click(screen.getByRole("checkbox", { name: "Select all visible reports" }));
    await user.click(screen.getByRole("button", { name: "Bulk Approve" }));

    await waitFor(() => expect(h.toggleComplete).toHaveBeenCalledWith(2, ""));
    expect(h.toggleComplete).not.toHaveBeenCalledWith(1, "");
    expect(h.toast.success).toHaveBeenCalledWith("1 approved, 1 skipped");
  });

  it("bulk unapproves only selected approved reports and reports skipped rows", async () => {
    h.listReports.mockResolvedValue(page([report(1, { completed: true }), report(2)]));
    h.toggleComplete.mockResolvedValue({ id: 1, completed: false });
    const { user } = renderReports("reports:read", "reports:approve");
    await screen.findByText("Worker 1");

    await user.click(screen.getByRole("checkbox", { name: "Select all visible reports" }));
    await user.click(screen.getByRole("button", { name: "Bulk Unapprove" }));
    expect(within(screen.getByRole("alertdialog")).getByRole("button", { name: "Bulk Unapprove" })).toBeDisabled();
    await user.type(screen.getByRole("textbox", { name: "Admin Notes" }), "Needs updated totals");
    await user.click(within(screen.getByRole("alertdialog")).getByRole("button", { name: "Bulk Unapprove" }));

    await waitFor(() => expect(h.toggleComplete).toHaveBeenCalledWith(1, "Needs updated totals"));
    expect(h.toggleComplete).not.toHaveBeenCalledWith(2, "");
    expect(h.toast.success).toHaveBeenCalledWith("1 unapproved, 1 skipped");
  });

  it("neither is there without reports:approve", async () => {
    h.listReports.mockResolvedValue(page([report(1, { completed: true }), report(2)]));
    renderReports("reports:read");
    await screen.findByText("Worker 1");
    expect(screen.queryByRole("button", { name: /^(Approve|Unapprove) / })).not.toBeInTheDocument();
  });

  it("Unapprove toggles once, holds the button, then refreshes the list and the stats cards", async () => {
    let resolve!: (v: unknown) => void;
    h.toggleComplete.mockReturnValue(new Promise((r) => (resolve = r)));
    h.listReports.mockResolvedValue(page([report(1, { completed: true }), report(2)]));
    const { user, client } = renderReports("reports:read", "reports:approve");
    const invalidate = vi.spyOn(client, "invalidateQueries");
    await screen.findByText("Worker 1");

    await user.click(unapprove("Worker 1"));
    expect(h.toggleComplete).not.toHaveBeenCalled();
    await user.type(screen.getByRole("textbox", { name: "Admin Notes" }), "Needs updated totals");
    await user.click(screen.getByRole("button", { name: "Unapprove" }));
    expect(h.toggleComplete).toHaveBeenCalledTimes(1);
    expect(h.toggleComplete).toHaveBeenCalledWith(1, "Needs updated totals");
    expect(unapprove("Worker 1")).toBeDisabled();

    h.listReports.mockResolvedValue(page([report(1), report(2)]));
    resolve({ id: 1, completed: false });
    await waitFor(() => expect(h.toast.success).toHaveBeenCalledWith("Report unapproved: September 2026"));
    expect(invalidate).toHaveBeenCalledWith({ queryKey: ["reports"] });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: ["admin-stats"] });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: ["tracker"] });
    await waitFor(() => expect(within(row("Worker 1")).getByText("Pending")).toBeInTheDocument());
    expect(approve("Worker 1")).toBeEnabled();
  });

  it("a failed Unapprove leaves the row Approved and does not touch the cards", async () => {
    h.toggleComplete.mockRejectedValue(new ApiError(500, "ERROR", "Server error."));
    h.listReports.mockResolvedValue(page([report(1, { completed: true })]));
    const { user, client } = renderReports("reports:read", "reports:approve");
    const invalidate = vi.spyOn(client, "invalidateQueries");
    await screen.findByText("Worker 1");
    await user.click(unapprove("Worker 1"));
    await user.type(screen.getByRole("textbox", { name: "Admin Notes" }), "Needs updated totals");
    await user.click(screen.getByRole("button", { name: "Unapprove" }));
    await waitFor(() => expect(h.toast.error).toHaveBeenCalledWith("Server error."));
    expect(within(row("Worker 1")).getByText("Approved")).toBeInTheDocument();
    expect(unapprove("Worker 1")).toBeEnabled();
    expect(invalidate).not.toHaveBeenCalledWith({ queryKey: ["admin-stats"] });
  });

  it("toggles once, holds the button while it waits, then refreshes the list and the stats cards", async () => {
    let resolve!: (v: unknown) => void;
    h.toggleComplete.mockReturnValue(new Promise((r) => (resolve = r)));
    const { user, client } = renderReports("reports:read", "reports:approve");
    const invalidate = vi.spyOn(client, "invalidateQueries");
    await screen.findByText("Worker 1");

    await user.click(approve("Worker 1"));
    expect(h.toggleComplete).toHaveBeenCalledTimes(1);
    expect(h.toggleComplete).toHaveBeenCalledWith(1, "");
    expect(approve("Worker 1")).toBeDisabled(); // a second click would undo the first: the endpoint toggles

    h.listReports.mockResolvedValue(page([report(1, { completed: true }), report(2)]));
    resolve({ id: 1, completed: true });
    await waitFor(() => expect(h.toast.success).toHaveBeenCalledWith("Report approved: September 2026"));
    expect(invalidate).toHaveBeenCalledWith({ queryKey: ["reports"] });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: ["admin-stats"] });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: ["tracker"] });
    // the refetched list now shows it done, with no button
    await waitFor(() => expect(within(row("Worker 1")).getByText("Approved")).toBeInTheDocument());
    expect(within(row("Worker 1")).queryByRole("button", { name: /Approve/ })).not.toBeInTheDocument();
  });

  it("holds each row on its own while two approvals are in flight", async () => {
    const resolves: ((v: unknown) => void)[] = [];
    h.toggleComplete.mockImplementation(() => new Promise((r) => resolves.push(r)));
    const { user } = renderReports("reports:read", "reports:approve");
    await screen.findByText("Worker 1");
    await user.click(approve("Worker 1"));
    await user.click(approve("Worker 2"));
    expect(approve("Worker 1")).toBeDisabled();
    expect(approve("Worker 2")).toBeDisabled();

    resolves[1]({ id: 2, completed: true }); // the second answers first
    await waitFor(() => expect(approve("Worker 2")).toBeEnabled());
    expect(approve("Worker 1")).toBeDisabled();

    resolves[0]({ id: 1, completed: true });
    await waitFor(() => expect(approve("Worker 1")).toBeEnabled());
    expect(h.toast.success).toHaveBeenCalledTimes(2);
  });

  it("tells the truth when the server unapproved it instead (it was already approved elsewhere)", async () => {
    h.toggleComplete.mockResolvedValue({ id: 1, completed: false });
    const { user } = renderReports("reports:read", "reports:approve");
    await screen.findByText("Worker 1");
    await user.click(approve("Worker 1"));
    await waitFor(() => expect(h.toast.success).toHaveBeenCalledWith("Report unapproved: September 2026"));
  });

  it("a 404 says the report is gone and reloads the list, not the cards", async () => {
    h.toggleComplete.mockRejectedValue(new ApiError(404, "NOT_FOUND", "gone"));
    const { user, client } = renderReports("reports:read", "reports:approve");
    const invalidate = vi.spyOn(client, "invalidateQueries");
    await screen.findByText("Worker 1");
    await user.click(approve("Worker 1"));
    await waitFor(() => expect(h.toast.error).toHaveBeenCalledWith("September 2026 no longer exists."));
    await waitFor(() => expect(h.listReports).toHaveBeenCalledTimes(2));
    expect(invalidate).not.toHaveBeenCalledWith({ queryKey: ["admin-stats"] });
  });

  it("another error shows its message with the retry hint, leaves the row, and does not touch the cards", async () => {
    h.toggleComplete.mockRejectedValue(new ApiError(429, "RATE_LIMIT", "Slow down.", undefined, 9));
    const { user, client } = renderReports("reports:read", "reports:approve");
    const invalidate = vi.spyOn(client, "invalidateQueries");
    await screen.findByText("Worker 1");
    await user.click(approve("Worker 1"));
    await waitFor(() => expect(h.toast.error).toHaveBeenCalledWith("Slow down. Try again in 9s."));
    expect(within(row("Worker 1")).getByText("Pending")).toBeInTheDocument();
    expect(approve("Worker 1")).toBeEnabled();
    expect(invalidate).not.toHaveBeenCalledWith({ queryKey: ["admin-stats"] });
  });

  it("a failure that is not an API error gets a generic message", async () => {
    h.toggleComplete.mockRejectedValue(new Error("network down"));
    const { user } = renderReports("reports:read", "reports:approve");
    await screen.findByText("Worker 1");
    await user.click(approve("Worker 1"));
    await waitFor(() => expect(h.toast.error).toHaveBeenCalledWith("Could not update the report. Try again."));
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
    h.listReports.mockResolvedValue(page([report(1, { owner_status, owner_deleted_at })]));
    renderReports("reports:read");
    await screen.findByText("Worker 1");
    const workerRow = within(row("Worker 1"));
    expect(workerRow.queryByText("Deleted user") !== null).toBe(deleted);
    expect(workerRow.queryByText("Archived") !== null).toBe(archived);
    expect(workerRow.getByText("Pending")).toBeInTheDocument();
  });
});
