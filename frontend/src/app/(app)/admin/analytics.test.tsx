import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { AttendanceAnalytics } from "@/lib/analytics";
import { Analytics } from "./analytics";

const h = vi.hoisted(() => ({
  getAttendanceAnalytics: vi.fn(),
  getStats: vi.fn(),
  exportAttendanceAnalytics: vi.fn(),
  saveFile: vi.fn(),
}));
vi.mock("@/lib/analytics", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/analytics")>()),
  getAttendanceAnalytics: h.getAttendanceAnalytics,
  exportAttendanceAnalytics: h.exportAttendanceAnalytics,
}));
vi.mock("@/lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/api")>()),
  saveFile: h.saveFile,
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
    { metric: "worship_service", group: "Attendance", total: 26, churches: [{ church_name: "Bethel", value: 26 }] },
    { metric: "tithes_offering", group: "Finance", total: 120.5, churches: [{ church_name: "Bethel", value: 120.5 }] },
    { metric: "baptism", group: "Evangelism & Growth", total: null, churches: [{ church_name: "Bethel", value: null }] },
  ],
  no_report_churches: ["Zion"],
  weeks: 2,
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

afterEach(() => vi.restoreAllMocks());

describe("analytics", () => {
  it("loads with current-year, Worship Service, and Approved defaults", async () => {
    renderAnalytics();
    expect(screen.getByRole("status", { name: "Loading analytics" })).toBeInTheDocument();
    await screen.findByText("Monthly trend");
    expect(screen.getByLabelText("Analytics scope")).toHaveTextContent("Full Year 2026 · Approved reports · 1 of 2 churches reporting".replace("2026", String(new Date().getFullYear())));
    // The KPI cards cover every metric, whatever the trend shows.
    const summary = screen.getByRole("region", { name: "Analytics summary" });
    expect(within(summary).getByText("13")).toBeInTheDocument(); // 26 worship ÷ 2 weeks
    expect(within(summary).getByText("₱120.50")).toBeInTheDocument();
    expect(within(summary).getByText("1 of 2")).toBeInTheDocument();
    const trendSummary = screen.getByRole("region", { name: "Trend summary" });
    expect(within(trendSummary).getByText("26")).toBeInTheDocument();
    expect(within(trendSummary).getByText("January")).toBeInTheDocument();
    expect(within(trendSummary).getByText("1 out of 3 churches")).toBeInTheDocument();
    expect(screen.getByLabelText("Year")).toHaveValue(String(new Date().getFullYear()));
    expect(screen.getByLabelText("Trend for")).toHaveValue("worship_service");
    expect(screen.getByLabelText("Status")).toHaveValue("approved");
    expect(screen.getByLabelText("Period")).toHaveValue("full_year");
    expect(screen.getByLabelText("Compare with previous year")).not.toBeChecked();
    expect(h.getAttendanceAnalytics).toHaveBeenLastCalledWith(new Date().getFullYear(), "worship_service", "approved", "full_year", false);
  });

  it("refetches when filters change and exposes equivalent table data", async () => {
    const { user } = renderAnalytics();
    await screen.findByText("Monthly trend");
    await user.selectOptions(screen.getByLabelText("Trend for"), "tithes_offering");
    await user.selectOptions(screen.getByLabelText("Status"), "all");
    await user.selectOptions(screen.getByLabelText("Period"), "q2");
    await user.click(screen.getByLabelText("Compare with previous year"));
    expect(h.getAttendanceAnalytics).toHaveBeenLastCalledWith(new Date().getFullYear(), "tithes_offering", "all", "q2", true);

    // Weeks add up to months, and every month of the period is listed.
    const trendTable = screen.getByRole("table", { name: "Monthly trend data" });
    expect(within(trendTable).getByRole("row", { name: /January/ })).toHaveTextContent("26");
    expect(within(trendTable).getByRole("row", { name: /February/ })).toHaveTextContent("0");
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

    expect(await screen.findAllByText("+30% vs last year")).not.toHaveLength(0);
    expect(screen.getAllByText("2025").length).toBeGreaterThan(0);
    expect(screen.getByRole("table", { name: "Church breakdown data" })).toHaveTextContent("20");
  });

  it("renders the full metrics summary grouped, with every church and nullable currency values", async () => {
    renderAnalytics();

    const table = (await screen.findAllByRole("table", { name: "Full metrics summary" }))[0];
    const rows = within(table).getAllByRole("row");
    expect(within(rows[0]).getAllByRole("columnheader").map((cell) => cell.textContent)).toEqual(["Metric Name", "Total for Period", "Avg / Week", "Bethel", "Zion"]);
    expect(rows.slice(1).map((row) => row.firstChild?.textContent)).toEqual(["Attendance", "Worship Service", "Finance", "Tithes & Offering", "Evangelism & Growth", "Baptism"]);
    // Attendance gets an average per week; other metrics leave it blank.
    expect(within(table).getByRole("row", { name: /Worship Service/ })).toHaveTextContent("Worship Service261326No report");
    expect(within(table).getAllByText("₱120.50")).toHaveLength(2);
    expect(within(table).getAllByText("—")).toHaveLength(2);
    expect(within(table).getAllByText("No report")).toHaveLength(3);
  });

  it("renders the analytics sections once in the requested order", async () => {
    renderAnalytics();

    await screen.findByText("Monthly trend");
    expect(screen.getAllByRole("region", { name: "Analytics summary" })).toHaveLength(1);
    expect(screen.getAllByRole("table", { name: "Full metrics summary" })).toHaveLength(1);
    const sections = [
      screen.getByLabelText("Analytics scope"),
      screen.getByRole("region", { name: "Analytics summary" }),
      screen.getByRole("heading", { name: "Full Metrics Summary" }),
      screen.getByRole("heading", { name: "Trend" }),
      screen.getByLabelText("Trend for"),
      screen.getByText("Monthly trend"),
      screen.getByRole("table", { name: "Monthly trend data" }),
    ];
    sections.slice(0, -1).forEach((section, index) => {
      expect(Boolean(section.compareDocumentPosition(sections[index + 1]) & Node.DOCUMENT_POSITION_FOLLOWING)).toBe(true);
    });
  });

  it("downloads the server's Excel file for the selected filters", async () => {
    const file = { blob: new Blob(["PK"]), filename: "analytics-2026-q1.xlsx" };
    h.exportAttendanceAnalytics.mockResolvedValue(file);
    const { user } = renderAnalytics();
    await user.selectOptions(screen.getByLabelText("Period"), "q1");
    await user.click(await screen.findByRole("button", { name: "Export to Excel" }));

    expect(h.exportAttendanceAnalytics).toHaveBeenCalledWith(new Date().getFullYear(), "worship_service", "approved", "q1", false);
    await vi.waitFor(() => expect(h.saveFile).toHaveBeenCalledWith(file));
  });
});
