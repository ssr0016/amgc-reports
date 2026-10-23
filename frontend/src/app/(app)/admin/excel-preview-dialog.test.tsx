import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/api";
import { ExcelPreviewDialog, type PreviewTarget } from "./excel-preview-dialog";

const h = vi.hoisted(() => ({ previewExcel: vi.fn() }));
vi.mock("@/lib/reports", async (orig) => ({
  ...(await orig<typeof import("@/lib/reports")>()),
  previewExcel: h.previewExcel,
}));

const grid = (a1: string) => [[a1, "", "", "", "", "", ""], ["", "<b>not html</b>", "", "", "", "", ""]];

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
    const table = await screen.findByRole("table", { name: "Sheet Bethel" });
    expect(h.previewExcel).toHaveBeenCalledWith({ report_id: 7 });
    expect(within(table).getByText("Bethel title")).toBeInTheDocument();
    // Cell text is text, never markup.
    expect(within(table).getByText("<b>not html</b>")).toBeInTheDocument();
    expect(screen.queryByLabelText("Sheet")).not.toBeInTheDocument();
  });

  it("bulk opens on the Report Dashboard, six columns wide", async () => {
    renderDialog();
    const table = await screen.findByRole("table", { name: "Sheet Report Dashboard" });
    expect(screen.getByLabelText("Sheet")).toHaveDisplayValue("Report Dashboard");
    expect(within(table).getAllByRole("columnheader").map((c) => c.textContent)).toEqual(["Row", "A", "B", "C", "D", "E", "F"]);
    expect(within(table).getByText("September")).toBeInTheDocument();
    expect(within(table).getByText("No Report Submitted")).toBeInTheDocument();
    expect(screen.getByText("2 approved")).toBeInTheDocument();
  });

  it("a month with no approved reports shows the dashboard alone and can still download", async () => {
    h.previewExcel.mockResolvedValue({ count: 0, sheets: [dashboard(["Abba", "", "", "No Report Submitted", "", ""])] });
    renderDialog();
    await screen.findByRole("table", { name: "Sheet Report Dashboard" });
    expect(screen.getByText("0 approved")).toBeInTheDocument();
    expect(within(screen.getByLabelText("Sheet")).getAllByRole("option").map((o) => o.textContent)).toEqual(["Report Dashboard"]);
    expect(screen.getByRole("button", { name: "Download" })).toBeEnabled();
  });

  it("bulk shows one sheet at a time, chosen with Sheet, seven columns for a report", async () => {
    const { user } = renderDialog();
    await screen.findByRole("table", { name: "Sheet Report Dashboard" });
    await user.selectOptions(screen.getByLabelText("Sheet"), "Bethel");
    const bethel = screen.getByRole("table", { name: "Sheet Bethel" });
    expect(within(bethel).getAllByRole("columnheader")).toHaveLength(8);
    await user.selectOptions(screen.getByLabelText("Sheet"), "Zion");
    expect(screen.getByRole("table", { name: "Sheet Zion" })).toBeInTheDocument();
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
