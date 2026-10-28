import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { AttendanceAnalytics, MetricSummary } from "@/lib/analytics";
import { Analytics } from "./analytics";

const h = vi.hoisted(() => ({
  getAttendanceAnalytics: vi.fn(),
  exportAttendanceAnalytics: vi.fn(),
  exportAnalyticsCsv: vi.fn(),
  setChurchReporting: vi.fn(),
  listReports: vi.fn(),
  saveFile: vi.fn(),
  perms: new Set(["tracker:read"]),
}));
vi.mock("@/lib/analytics", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/analytics")>()),
  getAttendanceAnalytics: h.getAttendanceAnalytics,
  exportAttendanceAnalytics: h.exportAttendanceAnalytics,
  exportAnalyticsCsv: h.exportAnalyticsCsv,
  setChurchReporting: h.setChurchReporting,
}));
vi.mock("@/lib/reports", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/reports")>()),
  listReports: h.listReports,
}));
vi.mock("@/lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/api")>()),
  saveFile: h.saveFile,
}));
vi.mock("@/lib/session", () => ({ useSession: () => ({ can: (p: string) => h.perms.has(p) }) }));
vi.mock("recharts", () => {
  const Chart = ({ children }: { children?: React.ReactNode }) => <div>{children}</div>;
  return { Bar: Chart, BarChart: Chart, CartesianGrid: Chart, Line: Chart, LineChart: Chart, ResponsiveContainer: Chart, Tooltip: Chart, XAxis: Chart, YAxis: Chart };
});

const MONTHS = ["January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"];
const year = new Date().getFullYear();

function metric(key: string, label: string, group: string, unit: MetricSummary["unit"], total: number | null, churches: MetricSummary["churches"], extra: Partial<MetricSummary> = {}): MetricSummary {
  return {
    metric: key, label, group, unit, total, observations: total == null ? 0 : 3, zero_observations: 0, reports_with_value: 1, churches_with_value: 1,
    network_slots: 0, network_slots_full: 0, peak_month: "", peak_value: 0, churches, ...extra,
  };
}

// Bethel reported January (worship 40 + 0 + 50); Grace's January is pending; Zion has no report.
// February has no data at all; March onwards has not ended.
const body: AttendanceAnalytics = {
  year, metric: "worship_service", status: "approved", period: "full_year", generated_at: "2026-10-28T10:00:00+08:00", default_start_month: `${year}-01`,
  months: MONTHS.map((name, i) => ({ month: i + 1, name, state: i < 2 ? "closed" : "not_yet_open" })),
  cutoff_month: 2,
  church_options: [{ id: 1, name: "Bethel" }, { id: 2, name: "Grace" }, { id: 3, name: "Zion" }],
  churches: [
    { id: 1, name: "Bethel", start_month: `${year}-01`, start_is_default: true, needs_review: false, active: true, account_status: "active", workers: "Ana" },
    { id: 2, name: "Grace", start_month: `${year}-02`, start_is_default: false, needs_review: false, active: true, account_status: "active", workers: "Ben" },
    { id: 3, name: "Zion", start_month: `${year}-01`, start_is_default: true, needs_review: false, active: false, account_status: "inactive", workers: "Cy" },
  ],
  coverage: { months: 2, reports_in_scope: 1, expected_reported: 1, outside_reports: 0, churches_reported: 1, expected: 4, churches: 3 },
  metrics: [
    metric("worship_service", "Worship Service", "Attendance", "people", 90, [
      { church_id: 1, value: 90, observations: 3, state: "value" },
      { church_id: 2, value: null, observations: 0, state: "not_approved" },
      { church_id: 3, value: null, observations: 0, state: "no_report" },
    ], { avg_per_church_service: 30, network_weekly: 30, network_slots: 3, network_slots_full: 0 }),
    metric("tithes_offering", "Tithes & Offering", "Finance", "php", 120.5, [
      { church_id: 1, value: 120.5, observations: 1, state: "value" },
      { church_id: 2, value: null, observations: 0, state: "not_approved" },
      { church_id: 3, value: null, observations: 0, state: "no_report" },
    ]),
    metric("baptism", "Baptisms", "Evangelism & Growth", "count", null, [
      { church_id: 1, value: null, observations: 0, state: "blank" },
      { church_id: 2, value: null, observations: 0, state: "not_approved" },
      { church_id: 3, value: null, observations: 0, state: "no_report" },
    ]),
  ],
  trend: MONTHS.map((name, i) => ({
    month: i + 1, name, state: i < 2 ? "closed" : "not_yet_open", counted: i < 2, value: i === 0 ? 90 : null,
    observations: i === 0 ? 3 : 0, churches_with_value: i === 0 ? 1 : 0, churches_reported: i === 0 ? 1 : 0, expected_reported: i === 0 ? 1 : 0, churches_expected: i < 2 ? 2 : 0, partial: i < 2,
  })),
  slots: [{ month: 1, week: 1, total: 40, churches_with_value: 1, churches_reported: 1 }, { month: 1, week: 2, total: 0, churches_with_value: 1, churches_reported: 1 }],
  church_rows: [
    { church_id: 1, church_name: "Bethel", total: 90, observations: 3, average: 30, previous_observations: 0, tithes_total: 120.5, submitted: 1, approved: 1, expected: 2, completeness: 0.5 },
    { church_id: 2, church_name: "Grace", total: null, observations: 0, average: null, previous_observations: 0, tithes_total: null, submitted: 1, approved: 0, expected: 2, completeness: 0.5 },
    { church_id: 3, church_name: "Zion", total: null, observations: 0, average: null, previous_observations: 0, tithes_total: null, submitted: 0, approved: 0, expected: 0, completeness: null },
  ],
  compliance: [
    { church_id: 1, church_name: "Bethel", expected: 2, outside: 0, submitted: 1, approved: 1, pending: 0, needs_revision: 0, on_time: 1, late: 0, awaiting: 0, overdue: 0, not_filed: 1, duplicates: 0,
      cells: MONTHS.map((_, i) => i === 0 ? { month: 1, expected: true, state: "submitted", report_id: 11, review_status: "approved", timeliness: "on_time", submitted_on: `${year}-02-05` } : { month: i + 1, expected: i === 1, state: i === 1 ? "not_filed" : "not_yet_open" }) },
    { church_id: 2, church_name: "Grace", expected: 2, outside: 0, submitted: 1, approved: 0, pending: 1, needs_revision: 0, on_time: 0, late: 1, awaiting: 0, overdue: 0, not_filed: 1, duplicates: 0,
      cells: MONTHS.map((_, i) => i === 0 ? { month: 1, expected: true, state: "submitted", report_id: 12, review_status: "pending", timeliness: "late", submitted_on: `${year}-02-20` } : { month: i + 1, expected: i === 1, state: i === 1 ? "not_filed" : "not_yet_open" }) },
    { church_id: 3, church_name: "Zion", expected: 0, outside: 0, submitted: 0, approved: 0, pending: 0, needs_revision: 0, on_time: 0, late: 0, awaiting: 0, overdue: 0, not_filed: 0, duplicates: 0,
      cells: MONTHS.map((_, i) => ({ month: i + 1, expected: false, state: "inactive" })) },
  ],
  compliance_totals: { expected: 4, submitted: 2, approved: 1, pending: 1, needs_revision: 0, on_time: 1, late: 1, awaiting: 0, overdue: 0, not_filed: 2, duplicates: 0, churches_with_submission: 2, churches_needing_review: 0, outside: 0 },
};

function renderAnalytics() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const user = userEvent.setup();
  render(<QueryClientProvider client={client}><Analytics /></QueryClientProvider>);
  return { user };
}

const defaults = { year, metric: "worship_service", status: "approved", period: "full_year", compare: false, churches: [] };

beforeEach(() => {
  vi.clearAllMocks();
  h.perms = new Set(["tracker:read"]);
  h.getAttendanceAnalytics.mockResolvedValue(body);
});

afterEach(() => vi.restoreAllMocks());

describe("analytics", () => {
  it("loads with the default filters and labels each KPI with its denominator", async () => {
    renderAnalytics();
    expect(screen.getByRole("status", { name: "Loading analytics" })).toBeInTheDocument();
    await screen.findByText("Monthly trend");
    expect(h.getAttendanceAnalytics).toHaveBeenLastCalledWith(defaults);
    const scope = screen.getByLabelText("Analytics scope");
    // The count says which churches it covers, so it can be read against the Total Churches card.
    expect(scope).toHaveTextContent(`Full Year ${year} · Approved reports · All 3 churches (2 with an active church account, as on the Total Churches card; 1 inactive, listed for their reports or end month) · 1 reported`);
    expect(scope).toHaveTextContent("1 of 4 expected church-month reports counted");
    expect(scope).toHaveTextContent(/expected from Jan \d{4} unless a church has its own months/);
    expect(screen.getByLabelText("Analytics scope")).toHaveTextContent(`Counts January–February ${year} (year to date`);
    const summary = screen.getByRole("region", { name: "Analytics summary" });
    expect(within(summary).getByText("Avg worship attendance per church-service")).toBeInTheDocument();
    expect(within(summary).getByText("90 attendance entries ÷ 3 recorded church-services")).toBeInTheDocument();
    expect(within(summary).getByText("₱120.50")).toBeInTheDocument();
    expect(within(summary).getByText("2 of 4")).toBeInTheDocument(); // submitted / expected
    expect(within(summary).getByText("1 of 4")).toBeInTheDocument(); // approved / expected
    expect(within(summary).queryByText(/Active Churches/)).not.toBeInTheDocument();
  });

  it("keeps blanks, pending reports and missing reports apart in the matrix", async () => {
    renderAnalytics();
    const table = await screen.findByRole("table", { name: "Full metrics summary" });
    const worship = within(table).getByRole("row", { name: /Worship Service/ });
    expect(worship).toHaveTextContent("Not approved");
    expect(worship).toHaveTextContent("No report");
    const baptism = within(table).getByRole("row", { name: /Baptisms/ });
    expect(within(baptism).getAllByText("—")[0]).toBeInTheDocument(); // Bethel reported, left blank; total is no data
  });

  it("shows gaps, not zeros, for months without data", async () => {
    renderAnalytics();
    const trend = await screen.findByRole("table", { name: "Monthly trend data" });
    expect(within(trend).getByRole("row", { name: /January/ })).toHaveTextContent("1 / 2 (partial)");
    expect(within(trend).getByRole("row", { name: /February/ })).toHaveTextContent("No data");
    expect(within(trend).getByRole("row", { name: /March/ })).toHaveTextContent("Not yet due");
    const slots = screen.getByRole("table", { name: "Reporting-week slots" });
    expect(within(slots).getByRole("row", { name: /Week 2/ })).toHaveTextContent("0"); // an explicit zero stays 0
  });

  it("shows compliance with a legend and pending as submitted, not missing", async () => {
    renderAnalytics();
    const matrix = await screen.findByRole("table", { name: "Submission compliance by church and month" });
    expect(within(matrix).getByRole("row", { name: /Grace/ })).toHaveTextContent("Pending · late");
    expect(within(matrix).getByRole("row", { name: /Bethel/ })).toHaveTextContent("Not Filed");
    expect(within(matrix).getByRole("row", { name: /Zion/ })).toHaveTextContent("Church Inactive");
    expect(within(matrix).getByRole("row", { name: /Zion/ })).toHaveTextContent("(default) – ongoing · inactive");
    expect(within(matrix).getByRole("row", { name: /Grace/ })).not.toHaveTextContent("(default)");
    expect(screen.getByRole("list", { name: "Compliance legend" })).toHaveTextContent("Not Yet Open");
    expect(screen.getByRole("list", { name: "Compliance legend" })).not.toHaveTextContent("Start Not Set");
    expect(within(matrix).queryByRole("button", { name: /Edit reporting months/ })).not.toBeInTheDocument();
  });

  it("lets an account admin override one church's months; blank keeps the organization default", async () => {
    h.perms = new Set(["tracker:read", "users:write"]);
    h.setChurchReporting.mockResolvedValue({});
    const { user } = renderAnalytics();
    await user.click(await screen.findByRole("button", { name: "Edit reporting months for Zion" }));
    const first = screen.getByLabelText(/First month \(blank: Jan \d{4}\)/);
    expect(first).toHaveValue(""); // the default is not copied into an override
    await user.type(screen.getByLabelText("Last month (optional)"), `${year}-03`);
    await user.click(screen.getByRole("button", { name: "Save" }));
    expect(h.setChurchReporting).toHaveBeenCalledWith(3, { start_month: "", end_month: `${year}-03` });
  });

  it("says coverage is incomplete while a church's eligibility is unknown", async () => {
    h.getAttendanceAnalytics.mockResolvedValue({
      ...body,
      default_start_month: "",
      coverage: { ...body.coverage, reports_in_scope: 2, expected_reported: 0, outside_reports: 2, expected: 0 },
      compliance_totals: { ...body.compliance_totals, churches_needing_review: 3 },
    });
    renderAnalytics();
    const scope = await screen.findByLabelText("Analytics scope");
    expect(scope).toHaveTextContent("no church-month reports are expected in the counted months");
    expect(scope).toHaveTextContent("no organization start month is configured");
    expect(scope).toHaveTextContent("3 churches have no start month and are not counted as expected, so coverage is incomplete");
    expect(scope).not.toHaveTextContent("0 of 0");
    expect(scope).toHaveTextContent("(+2 from months not required, included in totals)");
  });

  it("shows the church breakdown chart beside the trend, keeping churches with no data", async () => {
    renderAnalytics();
    expect(await screen.findByRole("img", { name: "Worship Service church breakdown chart" })).toBeInTheDocument();
    expect(screen.getByText(`No data for ${year}: Zion.`)).toBeInTheDocument();
    expect(screen.getByText("Submitted but not yet approved: Grace (pending approval).")).toBeInTheDocument();
    const table = screen.getByRole("table", { name: "Church comparison" });
    expect(within(table).getByRole("columnheader", { name: /Submission completeness/ })).toBeInTheDocument();
    // A filed report outside the Approved filter shows its review status; only Zion has no data.
    expect(within(table).getByRole("row", { name: /Grace/ })).toHaveTextContent("Pending approval");
    expect(within(table).getByRole("row", { name: /Zion/ })).toHaveTextContent("No data");
  });

  it("shows comparisons as unavailable rather than 0% or infinite", async () => {
    h.getAttendanceAnalytics.mockResolvedValue({
      ...body,
      comparison: {
        year: year - 1,
        coverage: { months: 2, reports_in_scope: 3, expected_reported: 3, outside_reports: 0, churches_reported: 3, expected: 4, churches: 3 },
        metrics: [metric("worship_service", "Worship Service", "Attendance", "people", 60, [], { avg_per_church_service: 20 }), metric("tithes_offering", "Tithes & Offering", "Finance", "php", 0, [])],
        trend: body.trend,
        slots: [],
        like_for_like: { churches: 1, current_total: 90, current_observations: 3, previous_total: 60, previous_observations: 3, change: 0.5 },
      },
      church_rows: body.church_rows.map((r) => ({ ...r, change: r.church_id === 1 ? 0.5 : null })),
    });
    renderAnalytics();
    const summary = await screen.findByRole("region", { name: "Analytics summary" });
    expect(within(summary).getByText(`vs ${year - 1}: +50%`)).toBeInTheDocument();
    expect(within(summary).getByText(`vs ${year - 1}: unavailable (previous was 0)`)).toBeInTheDocument();
    expect(within(summary).getAllByText(`vs ${year - 1}: unavailable (no data)`).length).toBeGreaterThan(0);
    expect(summary).toHaveTextContent("Coverage differs between years (1 of 4 vs 3 of 4 expected reports counted)");
    const churches = screen.getByRole("table", { name: "Church comparison" });
    expect(within(churches).getByRole("row", { name: /Grace/ })).toHaveTextContent("n/a");
  });

  it("filters churches, sorts the comparison and opens a church", async () => {
    h.perms = new Set(["tracker:read", "reports:read_any"]);
    h.listReports.mockResolvedValue({ data: [
      { id: 11, month: "January", year, church_name: "Bethel", narrative_report: "Revival week", challenges: "", prayer_request: "Rain" },
      { id: 99, month: "January", year, church_name: "Bethel North", narrative_report: "Other church" },
    ], meta: {} });
    const { user } = renderAnalytics();
    await screen.findByText("Monthly trend");
    await user.click(screen.getByText("All churches (3)"));
    await user.click(screen.getByRole("checkbox", { name: "Grace" }));
    expect(h.getAttendanceAnalytics).toHaveBeenLastCalledWith({ ...defaults, churches: [2] });

    const table = screen.getByRole("table", { name: "Church comparison" });
    await user.click(within(table).getByRole("button", { name: "Total" }));
    const names = within(table).getAllByRole("row").slice(1).map((row) => row.firstChild?.textContent);
    expect(names).toEqual(["Bethel", "Grace", "Zion"]); // no data sorts last, but is still listed

    await user.click(within(table).getByRole("button", { name: "Bethel" }));
    const dialog = await screen.findByRole("alertdialog");
    expect(within(dialog).getByRole("link", { name: "Open report #11" })).toHaveAttribute("href", "/reports/11");
    expect(await within(dialog).findByText("Revival week")).toBeInTheDocument();
    expect(within(dialog).queryByText("Other church")).not.toBeInTheDocument();
    expect(h.listReports).toHaveBeenCalledWith({ church: "Bethel", year, limit: 100 });
  });

  it("downloads Excel and both CSVs with the selected filters", async () => {
    const file = { blob: new Blob(["x"]), filename: "amgc.csv" };
    h.exportAttendanceAnalytics.mockResolvedValue(file);
    h.exportAnalyticsCsv.mockResolvedValue(file);
    const { user } = renderAnalytics();
    await user.selectOptions(screen.getByLabelText("Period"), "q1");
    await screen.findByText("Monthly trend");
    for (const item of ["Excel workbook (all sheets)", "CSV: raw weekly detail", "CSV: submission compliance"]) {
      await user.click(screen.getByRole("button", { name: "Export" }));
      await user.click(await screen.findByRole("menuitem", { name: item }));
    }
    const filters = { ...defaults, period: "q1" };
    expect(h.exportAttendanceAnalytics).toHaveBeenCalledWith(filters);
    expect(h.exportAnalyticsCsv).toHaveBeenCalledWith(filters, "weekly_detail");
    expect(h.exportAnalyticsCsv).toHaveBeenCalledWith(filters, "compliance");
    await vi.waitFor(() => expect(h.saveFile).toHaveBeenCalledTimes(3));
  });
});
