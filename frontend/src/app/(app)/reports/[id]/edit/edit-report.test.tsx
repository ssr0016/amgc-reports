import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/api";
import { getReport, updateReport } from "@/lib/reports";
import { EditReport } from "./edit-report";

const h = vi.hoisted(() => ({ push: vi.fn(), replace: vi.fn(), toast: { success: vi.fn() }, userId: 7, search: "" }));
vi.mock("next/navigation", () => ({
  useParams: () => ({ id: "12" }),
  useRouter: () => ({ push: h.push, replace: h.replace }),
  useSearchParams: () => new URLSearchParams(h.search),
}));
vi.mock("sonner", () => ({ toast: h.toast }));
vi.mock("@/lib/session", () => ({ useSession: () => ({ user: { user_id: h.userId }, can: () => true }) }));
vi.mock("@/lib/reports", async (original) => ({
  ...(await original<typeof import("@/lib/reports")>()),
  getReport: vi.fn(),
  updateReport: vi.fn(),
}));
vi.mock("@/components/report-form", () => ({
  ReportForm: ({ onSubmit, endAction, initialUserNotes }: { onSubmit: (input: unknown) => Promise<void>; endAction?: React.ReactNode; initialUserNotes?: string }) => (
    <div>
      <output data-testid="initial-user-notes">{initialUserNotes}</output>
      <button onClick={() => void onSubmit({}).catch(() => undefined)}>Submit</button>
      {endAction}
    </div>
  ),
}));

const report = {
  id: 12,
  created_by: 7,
  month: "September",
  year: 2026,
  status: "pending",
  completed: false,
  owner_status: "active",
  metrics: {},
  created_at: "2026-10-01T00:00:00Z",
  updated_at: "2026-10-01T00:00:00Z",
  user_notes: "Please explain the corrected totals.",
} as never;

function renderEdit() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  vi.spyOn(client, "invalidateQueries");
  render(<QueryClientProvider client={client}><EditReport id={12} /></QueryClientProvider>);
  return client;
}

beforeEach(() => {
  vi.clearAllMocks();
  h.search = "";
  vi.mocked(getReport).mockResolvedValue(report);
  vi.mocked(updateReport).mockResolvedValue(report);
});

describe("EditReport", () => {
  it("shows a deleted state and Community Reports link when the report load returns 404", async () => {
    vi.mocked(getReport).mockRejectedValue(new ApiError(404, "NOT_FOUND", "Report not found"));
    renderEdit();
    expect(await screen.findByText("This report has been deleted")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Back to Community Reports" })).toHaveAttribute("href", "/reports");
  });

  it("refetches the report when the window regains focus", async () => {
    const client = renderEdit();
    await screen.findByRole("button", { name: "Submit" });
    expect((client.getQueryCache().find({ queryKey: ["report", 12] })?.options as { refetchOnWindowFocus?: boolean }).refetchOnWindowFocus).toBe(true);
  });

  it("saves a pending report with one PUT", async () => {
    renderEdit();
    await userEvent.click(await screen.findByRole("button", { name: "Submit" }));
    await waitFor(() => expect(h.replace).toHaveBeenCalledWith("/reports"));
    expect(updateReport).toHaveBeenCalledOnce();
    expect(updateReport).toHaveBeenCalledWith(12, {});
    expect(h.toast.success).toHaveBeenCalledWith("Report saved");
  });

  it("resubmits a Needs Revision report with the save alone", async () => {
    vi.mocked(getReport).mockResolvedValue({ ...(report as object), status: "needs_revision" } as never);
    renderEdit();
    await screen.findByRole("button", { name: "Submit" });
    await userEvent.click(screen.getByRole("button", { name: "Submit" }));
    await waitFor(() => expect(h.replace).toHaveBeenCalledWith("/reports"));
    expect(updateReport).toHaveBeenCalledWith(12, {});
    expect(h.toast.success).toHaveBeenCalledWith("Report resubmitted");
    expect(screen.queryByText("This report has been deleted")).not.toBeInTheDocument();
  });

  it("hydrates revision notes from the server into the form", async () => {
    renderEdit();
    expect(await screen.findByTestId("initial-user-notes")).toHaveTextContent("Please explain the corrected totals.");
  });

  it("turns a Submit 404 into a deleted state and invalidates stale caches", async () => {
    vi.mocked(updateReport).mockRejectedValue(new ApiError(404, "NOT_FOUND", "gone"));
    const client = renderEdit();
    await userEvent.click(await screen.findByRole("button", { name: "Submit" }));
    await waitFor(() => expect(screen.getByText("This report has been deleted")).toBeInTheDocument());
    expect(client.invalidateQueries).toHaveBeenCalledWith({ queryKey: ["report", 12] });
    expect(client.invalidateQueries).toHaveBeenCalledWith({ queryKey: ["current-report", 7] });
    expect(client.invalidateQueries).toHaveBeenCalledWith({ queryKey: ["reports"] });
    expect(h.replace).not.toHaveBeenCalled();
  });

  it("does not offer delete from the edit form", async () => {
    renderEdit();
    await screen.findByRole("button", { name: "Submit" });
    expect(screen.queryByRole("button", { name: "Delete Report" })).not.toBeInTheDocument();
  });

  it("returns to the page in ?from= after a save", async () => {
    h.search = `from=${encodeURIComponent("/admin?tab=reports")}`;
    renderEdit();
    await userEvent.click(await screen.findByRole("button", { name: "Submit" }));
    await waitFor(() => expect(h.replace).toHaveBeenCalledWith("/admin?tab=reports"));
  });

  it("ignores a ?from= that leaves the site", async () => {
    h.search = `from=${encodeURIComponent("//evil.example/x")}`;
    renderEdit();
    await userEvent.click(await screen.findByRole("button", { name: "Submit" }));
    await waitFor(() => expect(h.replace).toHaveBeenCalledWith("/reports"));
  });

  it("the deleted state links back to ?from=", async () => {
    h.search = `from=${encodeURIComponent("/admin?tab=reports")}`;
    vi.mocked(getReport).mockRejectedValue(new ApiError(404, "NOT_FOUND", "Report not found"));
    renderEdit();
    expect(await screen.findByRole("link", { name: "Back to All Reports" })).toHaveAttribute("href", "/admin?tab=reports");
  });
});
