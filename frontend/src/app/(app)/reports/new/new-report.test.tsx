import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { NewReport } from "./new-report";

const session = vi.hoisted(() => ({ role: "user", write: true }));
const reports = vi.hoisted(() => ({ getCurrentReport: vi.fn(), createReport: vi.fn(), toast: { success: vi.fn() } }));
const navigation = vi.hoisted(() => ({ push: vi.fn(), replace: vi.fn() }));
vi.mock("next/navigation", () => ({ useRouter: () => navigation }));
vi.mock("sonner", () => ({ toast: reports.toast }));
vi.mock("@/lib/reports", async () => ({
  ...(await vi.importActual<typeof import("@/lib/reports")>("@/lib/reports")),
  getCurrentReport: reports.getCurrentReport,
  createReport: reports.createReport,
}));
vi.mock("@/lib/session", () => ({
  useSession: () => ({ user: { role: session.role, name: "Ana", area_assignment: "North", church_name: "AMGC Church" }, can: () => session.write }),
}));

describe("new report access", () => {
  afterEach(() => vi.useRealTimers());

  beforeEach(() => {
    Object.assign(session, { role: "user", write: true });
    navigation.replace.mockReset();
    reports.getCurrentReport.mockResolvedValue(null);
    reports.createReport.mockResolvedValue({ id: 21 });
  });

  it.each([
    ["user", true, true],
    ["user", false, false],
    ["editor", true, false],
    ["admin", true, false],
    ["manager", true, false],
    ["custom", true, false],
    ["", true, false],
  ])("role %s with write=%s: can submit=%s", async (role, write, allowed) => {
    session.role = role;
    session.write = write;
    render(<QueryClientProvider client={new QueryClient()}><NewReport /></QueryClientProvider>);
    if (allowed) {
      expect(await screen.findByRole("button", { name: "Submit Report" })).toBeInTheDocument();
      expect(screen.queryByRole("button", { name: "Save Draft" })).not.toBeInTheDocument();
    } else {
      expect(screen.queryByRole("button", { name: "Submit Report" })).not.toBeInTheDocument();
      expect(screen.getByText("You are not allowed to create reports.")).toBeInTheDocument();
    }
  });

  it("creates the report only on Submit, never while typing", async () => {
    render(<QueryClientProvider client={new QueryClient()}><NewReport /></QueryClientProvider>);
    const challenges = await screen.findByLabelText("Challenges");
    vi.useFakeTimers();
    fireEvent.change(challenges, { target: { value: "Not yet" } });
    await act(() => vi.advanceTimersByTimeAsync(5000));
    expect(reports.createReport).not.toHaveBeenCalled();
    vi.useRealTimers();
    fireEvent.click(screen.getByRole("button", { name: "Submit Report" }));
    await waitFor(() => expect(reports.createReport).toHaveBeenCalledWith(expect.objectContaining({ challenges: "Not yet" })));
    expect(reports.createReport.mock.calls[0]).toHaveLength(1);
  });

  it.each([
    ["pending", "You already submitted a report for September 2026.", "View Report", "/reports/13"],
    ["approved", "You already submitted a report for September 2026.", "View Report", "/reports/13"],
    ["needs_revision", "Your report for September 2026 needs revision. Please check the admin notes and update it.", "Revise Report", "/reports/13/edit"],
  ])("opens the %s report directly", async (status, _message, _label, href) => {
    reports.getCurrentReport.mockResolvedValue({ id: 13, month: "September", year: 2026, status });
    render(<QueryClientProvider client={new QueryClient()}><NewReport /></QueryClientProvider>);
    await waitFor(() => expect(navigation.replace).toHaveBeenCalledWith(href));
  });
});
