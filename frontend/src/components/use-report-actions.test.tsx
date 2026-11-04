import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/api";
import type { Report, ReportMetadata } from "@/lib/reports";
import { useReportActions } from "./use-report-actions";

const h = vi.hoisted(() => ({
  toggleComplete: vi.fn(),
  toast: { success: vi.fn(), error: vi.fn() },
}));

vi.mock("sonner", () => ({ toast: h.toast }));
vi.mock("@/lib/reports", async (orig) => ({
  ...(await orig<typeof import("@/lib/reports")>()),
  toggleComplete: h.toggleComplete,
}));

const report = (id: number, completed = false): Report =>
  ({ id, completed, status: completed ? "approved" : "pending" }) as Report;

function renderActions() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const invalidate = vi.spyOn(client, "invalidateQueries");
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  );
  return { ...renderHook(() => useReportActions(), { wrapper }), invalidate };
}

beforeEach(() => vi.resetAllMocks());
afterEach(() => vi.useRealTimers());

describe("useReportActions bulk updates", () => {
  it.each(["approve", "unapprove"] as const)("serializes %s requests and skips ineligible reports", async (action) => {
    const pending: ((value: ReportMetadata) => void)[] = [];
    h.toggleComplete.mockImplementation(() => new Promise<ReportMetadata>((resolve) => pending.push(resolve)));
    const { result } = renderActions();
    const completed = action === "unapprove";
    const reports = [report(1, completed), report(2, !completed), report(3, completed)];
    let mutation!: ReturnType<typeof result.current.bulkToggle.mutateAsync>;

    act(() => {
      mutation = result.current.bulkToggle.mutateAsync({ reports, action, adminNotes: "Reviewed" });
    });
    await waitFor(() => expect(h.toggleComplete).toHaveBeenCalledTimes(1));
    expect(h.toggleComplete).toHaveBeenNthCalledWith(1, 1, "Reviewed");

    await act(async () => pending[0]({ id: 1, completed: !completed } as ReportMetadata));
    await waitFor(() => expect(h.toggleComplete).toHaveBeenCalledTimes(2));
    expect(h.toggleComplete).toHaveBeenNthCalledWith(2, 3, "Reviewed");

    await act(async () => {
      pending[1]({ id: 3, completed: !completed } as ReportMetadata);
      expect(await mutation).toEqual({ action, changed: 2, failed: 0, skipped: 1 });
    });
    expect(h.toast.success).toHaveBeenCalledWith(`2 ${completed ? "unapproved" : "approved"}, 1 skipped`);
  });

  it("waits for Retry-After before retrying a 429 and resuming the queue", async () => {
    vi.useFakeTimers();
    h.toggleComplete
      .mockRejectedValueOnce(new ApiError(429, "RATE_LIMIT", "Slow down.", undefined, 2))
      .mockResolvedValue({ completed: true });
    const { result } = renderActions();
    let mutation!: ReturnType<typeof result.current.bulkToggle.mutateAsync>;

    await act(async () => {
      mutation = result.current.bulkToggle.mutateAsync({ reports: [report(1), report(2)], action: "approve" });
    });
    expect(h.toggleComplete).toHaveBeenCalledTimes(1);
    await act(async () => vi.advanceTimersByTimeAsync(1999));
    expect(h.toggleComplete).toHaveBeenCalledTimes(1);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1);
      expect(await mutation).toEqual({ action: "approve", changed: 2, failed: 0, skipped: 0 });
    });
    expect(h.toggleComplete.mock.calls).toEqual([[1, ""], [1, ""], [2, ""]]);
    expect(h.toast.success).toHaveBeenCalledWith("2 approved");
  });

  it("counts a repeated 429 and a network failure, then continues without replaying the network failure", async () => {
    vi.useFakeTimers();
    h.toggleComplete
      .mockRejectedValueOnce(new ApiError(429, "RATE_LIMIT", "Slow down."))
      .mockRejectedValueOnce(new ApiError(429, "RATE_LIMIT", "Slow down."))
      .mockRejectedValueOnce(new ApiError(0, "NETWORK", "Connection lost."))
      .mockResolvedValue({ completed: true });
    const { result, invalidate } = renderActions();
    let mutation!: ReturnType<typeof result.current.bulkToggle.mutateAsync>;

    await act(async () => {
      mutation = result.current.bulkToggle.mutateAsync({ reports: [report(1), report(2), report(3)], action: "approve" });
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1000);
      expect(await mutation).toEqual({ action: "approve", changed: 1, failed: 2, skipped: 0 });
    });
    expect(h.toggleComplete.mock.calls).toEqual([[1, ""], [1, ""], [2, ""], [3, ""]]);
    expect(h.toast.error).toHaveBeenCalledWith("1 approved, 2 failed");
    for (const key of ["reports", "admin-stats", "tracker", "analytics-attendance"]) {
      expect(invalidate).toHaveBeenCalledWith({ queryKey: [key] });
    }
  });
});
