import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/api";
import { ExcelPreviewDialog, type PreviewTarget } from "./excel-preview-dialog";

const h = vi.hoisted(() => ({ previewExcel: vi.fn(), getReport: vi.fn() }));
vi.mock("@/lib/reports", async (orig) => ({
  ...(await orig<typeof import("@/lib/reports")>()),
  previewExcel: h.previewExcel,
  getReport: h.getReport,
}));

const grid = (a1: string) => {
  const rows = Array.from({ length: 42 }, () => Array(7).fill(""));
  rows[0][0] = a1;
  rows[2][0] = "NATIONAL WORKERS' MONTHLY MINISTRY REPORT";
  rows[3] = ["Month of:", "September 2026", "", "", "", "", ""];
  rows[9] = ["Worship Service", "<b>not html</b>", "", "", "", "", "12.00"];
  return rows;
};

function renderDialog(target: PreviewTarget = { month: "May", year: 2026 }) {
  const onDownload = vi.fn().mockResolvedValue(true);
  const onClose = vi.fn();
  const user = userEvent.setup();
  render(
    <QueryClientProvider client={new QueryClient()}>
      <ExcelPreviewDialog target={target} onDownload={onDownload} onClose={onClose} />
    </QueryClientProvider>,
  );
  return { user, onDownload, onClose };
}

const dashboard = (...rows: string[][]) => ({
  name: "Report Dashboard",
  rows: [
    ["Report Dashboard", "", "", "", "", ""],
    ["Churches", "Month", "Year", "Status", "Coordinator's Remark", "Support Status"],
    ...rows,
  ],
});

beforeEach(() => {
  vi.clearAllMocks();
  h.getReport.mockResolvedValue({ status: "approved" });
  h.previewExcel.mockResolvedValue({
    count: 2,
    sheets: [
      dashboard(["Bethel", "September", "2026", "Approved", "Checked", "Ready for Pick Up"], ["Abba", "", "", "No Report Submitted", "", ""]),
      { name: "Bethel", rows: grid("Bethel title") },
      { name: "Zion", rows: grid("Zion title") },
    ],
  });
});

describe("Excel preview dialog", () => {
  it("says it is loading, with Download disabled", () => {
    h.previewExcel.mockReturnValue(new Promise(() => {}));
    renderDialog();
    expect(screen.getByRole("status")).toHaveTextContent("Loading preview…");
    expect(screen.getByRole("button", { name: "Download" })).toBeDisabled();
  });

  it("shows the server's message as an alert, with Download disabled", async () => {
    h.previewExcel.mockRejectedValue(new ApiError(422, "VALIDATION", "A bulk export holds at most 100 reports."));
    renderDialog();
    expect(await screen.findByRole("alert")).toHaveTextContent("A bulk export holds at most 100 reports.");
    expect(screen.getByRole("button", { name: "Download" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Cancel" })).toBeEnabled();
  });

  it("a single report shows its sheet as a table, without a sheet selector", async () => {
    h.previewExcel.mockResolvedValue({ count: 1, sheets: [{ name: "Bethel", rows: grid("Bethel title") }] });
    renderDialog({ reportId: 7, label: "Bethel" });
    const sheet = await screen.findByRole("region", { name: "Sheet Bethel" });
    expect(h.previewExcel).toHaveBeenCalledWith({ report_id: 7 });
    expect(within(sheet).getByText("Bethel title")).toBeInTheDocument();
    // Cell text is text, never markup.
    expect(within(sheet).getByText("<b>not html</b>")).toBeInTheDocument();
    expect(within(sheet).getByAltText("AMGC logo")).toBeInTheDocument();
    expect(sheet).toHaveClass("bg-white");
    expect(screen.queryByLabelText("Sheet")).not.toBeInTheDocument();
  });

  it("shows the single report status badge from the report data", async () => {
    h.previewExcel.mockResolvedValue({ count: 1, sheets: [{ name: "Bethel", rows: grid("Bethel title") }] });
    h.getReport.mockResolvedValue({ status: "needs_revision" });
    renderDialog({ reportId: 7, label: "Bethel" });

    expect(await screen.findByText("Needs Revision")).toBeInTheDocument();
  });

  it("bulk opens on a clean dashboard table without spreadsheet chrome", async () => {
    renderDialog();
    const table = await screen.findByRole("table", { name: "Sheet Report Dashboard" });
    expect(screen.getByLabelText("Sheet")).toHaveDisplayValue("Report Dashboard");
    expect(screen.getByLabelText("Sheet")).toHaveClass("bg-white", "text-slate-900");
    expect(within(table).getAllByRole("columnheader").map((c) => c.textContent)).toEqual([
      "Report Dashboard",
      "Churches",
      "Month",
      "Year",
      "Status",
      "Coordinator's Remark",
      "Support Status",
    ]);
    expect(within(table).getByText("September")).toBeInTheDocument();
    expect(within(table).getByText("No Report Submitted")).toBeInTheDocument();
    expect(within(table).getByText("Approved")).toBeInTheDocument();
    expect(screen.getByText("2 approved")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Cancel" })).toHaveClass("border-slate-700");
    expect(screen.getByRole("button", { name: "Download" })).toHaveClass("bg-slate-950");
  });

  it("a month with no approved reports shows the dashboard alone and can still download", async () => {
    h.previewExcel.mockResolvedValue({ count: 0, sheets: [dashboard(["Abba", "", "", "No Report Submitted", "", ""])] });
    renderDialog();
    await screen.findByRole("table", { name: "Sheet Report Dashboard" });
    expect(screen.getByText("0 approved")).toBeInTheDocument();
    expect(within(screen.getByLabelText("Sheet")).getAllByRole("option").map((o) => o.textContent)).toEqual(["Report Dashboard"]);
    expect(screen.getByRole("button", { name: "Download" })).toBeEnabled();
  });

  it("bulk shows the selected church with the individual report layout", async () => {
    const { user } = renderDialog();
    await screen.findByRole("table", { name: "Sheet Report Dashboard" });
    await user.selectOptions(screen.getByLabelText("Sheet"), "Bethel");
    const bethel = screen.getAllByRole("region", { name: "Sheet Bethel" })[0];
    expect(within(bethel).getByAltText("AMGC logo")).toBeInTheDocument();
    expect(within(bethel).getAllByRole("table")).toHaveLength(2);
    expect(within(bethel).getAllByRole("columnheader")).toHaveLength(14);
    await user.selectOptions(screen.getByLabelText("Sheet"), "Zion");
    expect(screen.getByRole("region", { name: "Sheet Zion" })).toBeInTheDocument();
    expect(screen.getByText("Zion title")).toBeInTheDocument();
    expect(screen.queryByText("Bethel title")).not.toBeInTheDocument();
  });

  it("Download runs once and closes when it succeeds", async () => {
    const { user, onDownload, onClose } = renderDialog();
    await screen.findByRole("table");
    await user.click(screen.getByRole("button", { name: "Download" }));
    expect(onDownload).toHaveBeenCalledTimes(1);
    await waitFor(() => expect(onClose).toHaveBeenCalledTimes(1));
  });

  it("a failed Download keeps it open", async () => {
    const { user, onDownload, onClose } = renderDialog();
    onDownload.mockResolvedValue(false);
    await screen.findByRole("table");
    await user.click(screen.getByRole("button", { name: "Download" }));
    await waitFor(() => expect(screen.getByRole("button", { name: "Download" })).toBeEnabled());
    expect(onClose).not.toHaveBeenCalled();
  });

  it("Cancel and Escape close without downloading", async () => {
    const { user, onDownload, onClose } = renderDialog();
    await screen.findByRole("table");
    await user.click(screen.getByRole("button", { name: "Cancel" }));
    await user.keyboard("{Escape}");
    expect(onClose).toHaveBeenCalled();
    expect(onDownload).not.toHaveBeenCalled();
  });
});
