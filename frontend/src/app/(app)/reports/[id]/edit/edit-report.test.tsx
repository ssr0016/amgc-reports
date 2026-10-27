import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/api";
import { getReport, submitReport, updateReport } from "@/lib/reports";
import { EditReport } from "./edit-report";

const h = vi.hoisted(() => ({ push: vi.fn(), replace: vi.fn(), toast: { success: vi.fn() }, userId: 7 }));
vi.mock("next/navigation", () => ({
  useParams: () => ({ id: "12" }),
  useRouter: () => ({ push: h.push, replace: h.replace }),
}));
vi.mock("sonner", () => ({ toast: h.toast }));
vi.mock("@/lib/session", () => ({ useSession: () => ({ user: { user_id: h.userId }, can: () => true }) }));
vi.mock("@/lib/reports", async (original) => ({
  ...(await original<typeof import("@/lib/reports")>()),
  getReport: vi.fn(),
  submitReport: vi.fn(),
  updateReport: vi.fn(),
}));
vi.mock("@/components/report-form", () => ({
  ReportForm: ({ onAutoSave, onSubmit }: { onAutoSave?: (input: unknown) => Promise<void>; onSubmit: (input: unknown) => Promise<void> }) => (
    <div>
      {onAutoSave && <button onClick={() => void onAutoSave({}).catch(() => undefined)}>Auto-save</button>}
      <button onClick={() => void onSubmit({}).catch(() => undefined)}>Submit</button>
    </div>
  ),
}));

const report = {
  id: 12,
  created_by: 7,
  month: "September",
  year: 2026,
  status: "draft",
  completed: false,
  owner_status: "active",
  metrics: {},
  created_at: "2026-10-01T00:00:00Z",
  updated_at: "2026-10-01T00:00:00Z",
} as never;

function renderEdit() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  vi.spyOn(client, "invalidateQueries");
  render(<QueryClientProvider client={client}><EditReport id={12} /></QueryClientProvider>);
  return client;
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(getReport).mockResolvedValue(report);
  vi.mocked(updateReport).mockResolvedValue(report);
  vi.mocked(submitReport).mockResolvedValue(report);
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

  it("updates an existing draft through autosave", async () => {
    renderEdit();
    await userEvent.click(await screen.findByRole("button", { name: "Auto-save" }));
    await waitFor(() => expect(updateReport).toHaveBeenCalledWith(12, {}));
    expect(submitReport).not.toHaveBeenCalled();
  });

  it.each(["Auto-save", "Submit"])("turns a %s 404 into a deleted state and invalidates stale caches", async (action) => {
    vi.mocked(updateReport).mockRejectedValue(new ApiError(404, "NOT_FOUND", "gone"));
    const client = renderEdit();
    await userEvent.click(await screen.findByRole("button", { name: action }));
    await waitFor(() => expect(screen.getByText("This report has been deleted")).toBeInTheDocument());
    expect(client.invalidateQueries).toHaveBeenCalledWith({ queryKey: ["report", 12] });
    expect(client.invalidateQueries).toHaveBeenCalledWith({ queryKey: ["current-report", 7] });
    expect(client.invalidateQueries).toHaveBeenCalledWith({ queryKey: ["reports"] });
    expect(submitReport).not.toHaveBeenCalled();
    expect(h.replace).not.toHaveBeenCalled();
  });
});
