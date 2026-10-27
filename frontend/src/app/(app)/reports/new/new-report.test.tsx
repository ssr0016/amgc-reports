import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import userEvent from "@testing-library/user-event";
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { NewReport } from "./new-report";

const session = vi.hoisted(() => ({ role: "user", write: true }));
const reports = vi.hoisted(() => ({ getCurrentReport: vi.fn(), deleteReport: vi.fn() }));
const navigation = vi.hoisted(() => ({ push: vi.fn(), replace: vi.fn() }));
vi.mock("next/navigation", () => ({ useRouter: () => navigation }));
vi.mock("@/lib/reports", async () => ({
  ...(await vi.importActual<typeof import("@/lib/reports")>("@/lib/reports")),
  getCurrentReport: reports.getCurrentReport,
  deleteReport: reports.deleteReport,
}));
vi.mock("@/lib/session", () => ({
  useSession: () => ({ user: { role: session.role, name: "Ana", area_assignment: "North", church_name: "AMGC Church" }, can: () => session.write }),
}));

describe("new report access", () => {
  beforeEach(() => {
    Object.assign(session, { role: "user", write: true });
    navigation.replace.mockReset();
    reports.getCurrentReport.mockResolvedValue(null);
    reports.deleteReport.mockResolvedValue(undefined);
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
      expect(await screen.findByRole("button", { name: "Save Draft" })).toBeInTheDocument();
      expect(await screen.findByRole("button", { name: "Submit Report" })).toBeInTheDocument();
    } else {
      expect(screen.queryByRole("button", { name: "Submit Report" })).not.toBeInTheDocument();
      expect(screen.getByText("You are not allowed to create reports.")).toBeInTheDocument();
    }
  });

  it("shows the draft decision card", async () => {
    reports.getCurrentReport.mockResolvedValue({ id: 12, month: "September", year: 2026, status: "draft" });
    render(<QueryClientProvider client={new QueryClient()}><NewReport /></QueryClientProvider>);
    expect(await screen.findByText("You have a draft for September 2026.")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Continue Draft" })).toHaveAttribute("href", "/reports/12/edit");
  });

  it.each([
    ["pending", "You already submitted a report for September 2026.", "View Report", "/reports/13"],
    ["approved", "You already submitted a report for September 2026.", "View Report", "/reports/13"],
    ["needs_revision", "Your report for September 2026 needs revision. Please check the admin notes and update it.", "Revise Report", "/reports/13/edit"],
  ])("shows the %s decision card", async (status, message, label, href) => {
    reports.getCurrentReport.mockResolvedValue({ id: 13, month: "September", year: 2026, status });
    render(<QueryClientProvider client={new QueryClient()}><NewReport /></QueryClientProvider>);
    expect(await screen.findByText(message)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: label })).toHaveAttribute("href", href);
  });

  it("confirms Start New before deleting the draft and opening the form", async () => {
    const user = userEvent.setup();
    reports.getCurrentReport.mockResolvedValue({ id: 12, month: "September", year: 2026, status: "draft" });
    render(<QueryClientProvider client={new QueryClient()}><NewReport /></QueryClientProvider>);
    await user.click(await screen.findByRole("button", { name: "Start New" }));
    expect(screen.getByRole("alertdialog")).toBeInTheDocument();
    expect(reports.deleteReport).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: "Delete draft" }));
    expect(reports.deleteReport).toHaveBeenCalledWith(12);
    expect(await screen.findByRole("button", { name: "Save Draft" })).toBeInTheDocument();
  });
});
