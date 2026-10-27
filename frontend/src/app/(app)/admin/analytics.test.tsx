import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { AttendanceAnalytics } from "@/lib/analytics";
import { Analytics, buildAnalyticsCsv } from "./analytics";

const h = vi.hoisted(() => ({ getAttendanceAnalytics: vi.fn(), getStats: vi.fn() }));
vi.mock("@/lib/analytics", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/analytics")>()),
  getAttendanceAnalytics: h.getAttendanceAnalytics,
}));
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
  period: "full_year",
  trend,
  churches: [{ church_name: "Bethel", value: 26 }],
  summary: [
    { metric: "worship_service", total: 26, churches: [{ church_name: "Bethel", value: 26 }] },
    { metric: "tithes_offering", total: 120.5, churches: [{ church_name: "Bethel", value: 120.5 }] },
    { metric: "baptism", total: null, churches: [{ church_name: "Bethel", value: null }] },
  ],
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
    expect(screen.getByLabelText("Period")).toHaveValue("full_year");
    expect(screen.getByLabelText("Compare with previous year")).not.toBeChecked();
    expect(h.getAttendanceAnalytics).toHaveBeenLastCalledWith(new Date().getFullYear(), "worship_service", "approved", "full_year", false);
  });

  it("refetches when filters change and exposes equivalent table data", async () => {
    const { user } = renderAnalytics();
    await screen.findByText("Weekly trend");
    await user.selectOptions(screen.getByLabelText("Metric"), "tithes_offering");
    await user.selectOptions(screen.getByLabelText("Status"), "all");
    await user.selectOptions(screen.getByLabelText("Period"), "q2");
    await user.click(screen.getByLabelText("Compare with previous year"));
    expect(h.getAttendanceAnalytics).toHaveBeenLastCalledWith(new Date().getFullYear(), "tithes_offering", "all", "q2", true);

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

  it("shows comparison values and KPI changes", async () => {
    h.getAttendanceAnalytics.mockResolvedValueOnce({
      ...body,
      comparison: {
        year: 2025,
        trend: [{ month: "January", week: 1, value: 10 }, { month: "January", week: 2, value: 10 }, { month: "February", week: 1, value: 0 }],
        churches: [{ church_name: "Bethel", value: 20 }],
      },
    });

    renderAnalytics();

    expect(await screen.findByText("+30% vs last year")).toBeInTheDocument();
    expect(screen.getAllByText("2025").length).toBeGreaterThan(0);
    expect(screen.getByRole("table", { name: "Church breakdown data" })).toHaveTextContent("20");
  });

  it("renders the full metrics summary with dynamic churches and nullable currency values", async () => {
    renderAnalytics();

    const table = await screen.findByRole("table", { name: "Full metrics summary" });
    expect(within(table).getByText("Worship Service")).toBeInTheDocument();
    expect(within(table).getByText("Tithes & Offering")).toBeInTheDocument();
    expect(within(table).getAllByText("Bethel").length).toBeGreaterThan(0);
    expect(within(table).getAllByText("₱120.50")).toHaveLength(2);
    expect(within(table).getAllByText("—")).toHaveLength(2);
  });

  it("builds a CSV with escaped KPI and table values", () => {
    const csvData = { ...body, churches: [{ church_name: 'Grace, "North"', value: 26 }] };
    const csv = buildAnalyticsCsv({
      data: csvData,
      metricLabel: "Worship Service",
      yearlyTotal: 26,
      peakMonth: ["January", 26],
      activeChurches: 1,
      totalChurches: 3,
    });

    expect(csv).toContain("KPI,Value");
    expect(csv).toContain("Total,26");
    expect(csv).toContain("Church breakdown");
    expect(csv).toContain('"Grace, ""North""",26');
  });
});
