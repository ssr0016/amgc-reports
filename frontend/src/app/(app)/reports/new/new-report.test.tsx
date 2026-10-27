import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { NewReport } from "./new-report";

const session = vi.hoisted(() => ({ role: "user", write: true }));
const reports = vi.hoisted(() => ({ listReports: vi.fn() }));
vi.mock("next/navigation", () => ({ useRouter: () => ({ push: vi.fn(), replace: vi.fn() }) }));
vi.mock("@/lib/reports", async () => ({
  ...(await vi.importActual<typeof import("@/lib/reports")>("@/lib/reports")),
  listReports: reports.listReports,
}));
vi.mock("@/lib/session", () => ({
  useSession: () => ({ user: { role: session.role, name: "Ana", area_assignment: "North", church_name: "AMGC Church" }, can: () => session.write }),
}));

describe("new report access", () => {
  beforeEach(() => {
    Object.assign(session, { role: "user", write: true });
    reports.listReports.mockResolvedValue({ data: [], meta: {} });
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

  it("offers the existing draft instead of rendering a second form", async () => {
    reports.listReports.mockResolvedValue({ data: [{ id: 12, status: "draft" }], meta: {} });
    render(<QueryClientProvider client={new QueryClient()}><NewReport /></QueryClientProvider>);
    expect(await screen.findByRole("heading", { name: "Draft already started" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Continue Draft" })).toHaveAttribute("href", "/reports/12/edit");
    expect(screen.queryByRole("button", { name: "Submit Report" })).not.toBeInTheDocument();
  });

  it("offers the existing report in view mode instead of rendering a second form", async () => {
    reports.listReports.mockResolvedValue({ data: [{ id: 13, status: "pending" }], meta: {} });
    render(<QueryClientProvider client={new QueryClient()}><NewReport /></QueryClientProvider>);
    expect(await screen.findByRole("heading", { name: "Report already exists" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Open Report" })).toHaveAttribute("href", "/reports/13");
    expect(screen.queryByRole("button", { name: "Submit Report" })).not.toBeInTheDocument();
  });
});
