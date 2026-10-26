import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { AttendanceAnalytics } from "@/lib/analytics";
import { Analytics } from "./analytics";

const h = vi.hoisted(() => ({ getAttendanceAnalytics: vi.fn(), getStats: vi.fn() }));
vi.mock("@/lib/analytics", () => ({ getAttendanceAnalytics: h.getAttendanceAnalytics }));
vi.mock("@/lib/stats", () => ({ getStats: h.getStats }));
vi.mock("@/lib/session", () => ({ useSession: () => ({ can: () => true }) }));
vi.mock("recharts", () => {
  const Chart = ({ children }: { children?: React.ReactNode }) => <div>{children}</div>;
  return { Bar: Chart, BarChart: Chart, CartesianGrid: Chart, Line: Chart, LineChart: Chart, ResponsiveContainer: Chart, Tooltip: Chart, XAxis: Chart, YAxis: Chart };
});

const trend = [
  { month: "January", week: 1, value: 12 },
  { month: "January", week: 2, value: 14 },
  { month: "February", week: 1, value: 0 },
];
const body: AttendanceAnalytics = {
  year: 2026,
  metric: "worship_service",
  status: "approved",
  trend,
  churches: [{ church_name: "Bethel", value: 26 }],
};

function renderAnalytics() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const user = userEvent.setup();
  render(<QueryClientProvider client={client}><Analytics /></QueryClientProvider>);
  return { user };
}

beforeEach(() => {
  vi.clearAllMocks();
  h.getAttendanceAnalytics.mockResolvedValue(body);
  h.getStats.mockResolvedValue({ churches_total: 3 });
});

describe("analytics", () => {
  it("loads with current-year, Worship Service, and Approved defaults", async () => {
    renderAnalytics();
    expect(screen.getByRole("status")).toHaveTextContent("Loading…");
    await screen.findByText("Weekly trend");
    const summary = screen.getByRole("region", { name: "Analytics summary" });
    expect(within(summary).getByText("26")).toBeInTheDocument();
    expect(within(summary).getByText("January")).toBeInTheDocument();
    expect(within(summary).getByText("1 out of 3 churches")).toBeInTheDocument();
    expect(screen.getByLabelText("Year")).toHaveValue(String(new Date().getFullYear()));
    expect(screen.getByLabelText("Metric")).toHaveValue("worship_service");
    expect(screen.getByLabelText("Status")).toHaveValue("approved");
    expect(h.getAttendanceAnalytics).toHaveBeenLastCalledWith(new Date().getFullYear(), "worship_service", "approved");
  });

  it("refetches when filters change and exposes equivalent table data", async () => {
    const { user } = renderAnalytics();
    await screen.findByText("Weekly trend");
    await user.selectOptions(screen.getByLabelText("Metric"), "tithes_offering");
    await user.selectOptions(screen.getByLabelText("Status"), "all");
    expect(h.getAttendanceAnalytics).toHaveBeenLastCalledWith(new Date().getFullYear(), "tithes_offering", "all");

    const trendTable = screen.getByRole("table", { name: "Weekly trend data" });
    expect(within(trendTable).getAllByText("January")).toHaveLength(2);
    expect(within(trendTable).getByText("12")).toBeInTheDocument();
    expect(within(trendTable).queryByText("February")).not.toBeInTheDocument();
    const churchTable = screen.getByRole("table", { name: "Church breakdown data" });
    expect(within(churchTable).getByText("Bethel")).toBeInTheDocument();
  });

  it("shows an empty state for charts with no non-zero values", async () => {
    h.getAttendanceAnalytics.mockResolvedValueOnce({
      ...body,
      trend: [{ month: "January", week: 1, value: 0 }],
      churches: [{ church_name: "Bethel", value: 0 }],
    });

    renderAnalytics();

    expect(await screen.findAllByText("No data available for this selection")).toHaveLength(2);
  });
});
