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

beforeEach(() => {
  vi.clearAllMocks();
  h.previewExcel.mockResolvedValue({
    count: 2,
    sheets: [
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

  it("bulk shows one sheet at a time, chosen with Sheet, and the report count", async () => {
    const { user } = renderDialog();
    await screen.findByRole("table", { name: "Sheet Bethel" });
    expect(screen.getByText("2 reports")).toBeInTheDocument();
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
