"use client";

import { useMutation, useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { toast } from "sonner";
import { ChartColumn, ChevronDown, Download } from "lucide-react";
import { Bar, BarChart, CartesianGrid, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import type { TooltipContentProps } from "recharts";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { Label } from "@/components/ui/label";
import { NativeSelect } from "@/components/ui/native-select";
import { Table, TableBody, TableCaption, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { ErrorRetry } from "@/components/error-retry";
import {
  ANALYTICS_PERIODS,
  exportAnalyticsCsv,
  exportAttendanceAnalytics,
  getAttendanceAnalytics,
  type AnalyticsFilters,
  type AnalyticsPeriod,
  type AttendanceAnalytics,
  type AttendanceStatus,
  type ChurchMetric,
  type CsvKind,
  type MetricSummary,
  monthLabel,
} from "@/lib/analytics";
import { METRICS } from "@/lib/report-form";
import { FIRST_YEAR } from "@/lib/reports";
import { saveFile } from "@/lib/api";
import { errorText } from "@/lib/auth-form";
import { useSession } from "@/lib/session";
import { EmptyState } from "@/components/empty-state";
import { AnalyticsSkeleton } from "@/components/loading-skeletons";
import { ComplianceMatrix } from "./analytics-compliance";
import { ChurchDetail } from "./analytics-church-detail";
import { changeText, formatPct, formatValue, missingText } from "./analytics-format";

const periodLabel = (period: string) => ANALYTICS_PERIODS.find((item) => item.value === period)?.label ?? period;
const metricOf = (metrics: MetricSummary[] | undefined, key: string) => metrics?.find((m) => m.metric === key);

export function Analytics() {
  const { can } = useSession();
  const allowed = can("tracker:read");
  const [year, setYear] = useState(() => new Date().getFullYear());
  const [period, setPeriod] = useState<AnalyticsPeriod>("full_year");
  const [metric, setMetric] = useState("worship_service");
  const [status, setStatus] = useState<AttendanceStatus>("approved");
  const [compare, setCompare] = useState(false);
  const [churches, setChurches] = useState<number[]>([]);
  const [detail, setDetail] = useState<number | null>(null);
  const filters: AnalyticsFilters = { year, metric, status, period, compare, churches };
  const { data, error, isPending, refetch } = useQuery({
    queryKey: ["analytics-attendance", filters],
    queryFn: () => getAttendanceAnalytics(filters),
    enabled: allowed,
    staleTime: 5 * 60_000, // the charts change only when a report does, and refreshAdminViews invalidates them then
  });

  const download = useMutation({
    mutationFn: (kind: "excel" | CsvKind) => (kind === "excel" ? exportAttendanceAnalytics(filters) : exportAnalyticsCsv(filters, kind)),
    onSuccess: (file) => {
      saveFile(file);
      toast.success(`Downloaded ${file.filename}`);
    },
    onError: (err) => toast.error(errorText(err, "Could not download the file. Try again.")),
  });

  if (!allowed) return <p className="text-sm text-muted-foreground">You are not allowed to view analytics.</p>;

  const currentYear = new Date().getFullYear();
  const years = Array.from({ length: Math.max(1, currentYear - FIRST_YEAR + 2) }, (_, i) => FIRST_YEAR + i);
  const statusText = status === "approved" ? "Approved reports" : "All submitted reports";

  return (
    <div className="flex flex-col gap-6">
      <header className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <h2 className="text-xl font-medium tracking-tight">Analytics</h2>
          <p className="mt-1 text-sm text-muted-foreground">Ministry activity, attendance, giving and submission completeness for the churches and period you choose.</p>
        </div>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button type="button" variant="outline" disabled={!data || download.isPending} aria-busy={download.isPending}>
              <Download aria-hidden="true" />{download.isPending ? "Downloading…" : "Export"}<ChevronDown aria-hidden="true" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem onSelect={() => download.mutate("excel")}>Excel workbook (all sheets)</DropdownMenuItem>
            <DropdownMenuItem onSelect={() => download.mutate("weekly_detail")}>CSV: raw weekly detail</DropdownMenuItem>
            <DropdownMenuItem onSelect={() => download.mutate("compliance")}>CSV: submission compliance</DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </header>

      <form onSubmit={(event) => event.preventDefault()} className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <div className="flex flex-col gap-2"><Label htmlFor="analytics-year">Year</Label><NativeSelect id="analytics-year" value={year} onChange={(event) => setYear(Number(event.target.value))}>{years.map((option) => <option key={option} value={option}>{option}</option>)}</NativeSelect></div>
        <div className="flex flex-col gap-2"><Label htmlFor="analytics-period">Period</Label><NativeSelect id="analytics-period" value={period} onChange={(event) => setPeriod(event.target.value as AnalyticsPeriod)}>{ANALYTICS_PERIODS.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}</NativeSelect></div>
        <div className="flex flex-col gap-2"><Label htmlFor="analytics-status">Status</Label><NativeSelect id="analytics-status" value={status} onChange={(event) => setStatus(event.target.value as AttendanceStatus)}><option value="approved">Approved</option><option value="all">All Submitted</option></NativeSelect></div>
        <ChurchFilter options={data?.church_options ?? []} selected={churches} onChange={setChurches} />
        <label htmlFor="analytics-compare" className="flex min-h-11 items-center gap-3 text-sm sm:col-span-2 lg:col-span-4"><input id="analytics-compare" type="checkbox" className="size-5 accent-[var(--primary)]" checked={compare} onChange={(event) => setCompare(event.target.checked)} /><span>Compare with the same months of the previous year</span></label>
      </form>

      {isPending ? <AnalyticsSkeleton /> : error ? <ErrorRetry message={error.message} onRetry={() => refetch()} /> : (
        <>
          <ScopeLine data={data} statusText={statusText} />
          <Kpis data={data} />
          <ComplianceMatrix data={data} canEdit={can("users:write")} onOpenChurch={setDetail} />
          <FullMetricsSummary data={data} statusText={statusText} onOpenChurch={setDetail} />
          <section aria-labelledby="analytics-trend-heading" className="flex flex-col gap-4 border-t pt-6">
            <div className="flex flex-wrap items-end justify-between gap-4">
              <div><h2 id="analytics-trend-heading" className="text-base font-medium">Activity detail</h2><p className="text-sm text-muted-foreground">Pick one activity to see it month by month, by reporting-week slot and church by church.</p></div>
              <div className="flex min-w-56 flex-col gap-2"><Label htmlFor="analytics-metric">Activity</Label><NativeSelect id="analytics-metric" value={metric} onChange={(event) => setMetric(event.target.value)}>{METRICS.map((option) => <option key={option.key} value={option.key}>{option.label}</option>)}</NativeSelect></div>
            </div>
            <MonthlyTrend data={data} />
            <ChurchComparison data={data} onOpenChurch={setDetail} />
            <WeekSlots data={data} />
          </section>
          {detail !== null && <ChurchDetail data={data} churchId={detail} canReadReports={can("reports:read_any")} onClose={() => setDetail(null)} />}
        </>
      )}
    </div>
  );
}

function ChurchFilter({ options, selected, onChange }: { options: { id: number; name: string }[]; selected: number[]; onChange: (ids: number[]) => void }) {
  const label = selected.length === 0 ? `All churches${options.length ? ` (${options.length})` : ""}` : `${selected.length} of ${options.length} churches`;
  return (
    <div className="flex flex-col gap-2">
      <span id="analytics-churches-label" className="text-sm leading-none font-medium select-none">Churches</span>
      <details className="group relative">
        <summary aria-labelledby="analytics-churches-label analytics-churches-value" className="flex h-11 cursor-pointer list-none items-center justify-between rounded-md border bg-background px-3 text-sm">
          <span id="analytics-churches-value">{label}</span><ChevronDown aria-hidden="true" className="size-4" />
        </summary>
        <fieldset className="absolute z-20 mt-1 max-h-80 w-72 overflow-y-auto rounded-md border bg-popover p-2 shadow-md">
          <legend className="sr-only">Churches to include</legend>
          <label className="flex min-h-9 items-center gap-2 text-sm font-medium"><input type="checkbox" className="size-4" checked={selected.length === 0} onChange={() => onChange([])} />All churches</label>
          {options.map((o) => (
            <label key={o.id} className="flex min-h-9 items-center gap-2 text-sm">
              <input type="checkbox" className="size-4" checked={selected.includes(o.id)} onChange={(e) => onChange(e.target.checked ? [...selected, o.id] : selected.filter((id) => id !== o.id))} />
              {o.name}
            </label>
          ))}
        </fieldset>
      </details>
    </div>
  );
}

function ScopeLine({ data, statusText }: { data: AttendanceAnalytics; statusText: string }) {
  const first = data.months[0];
  const counted = data.cutoff_month ? `Counts ${first.name}–${data.months.find((m) => m.month === data.cutoff_month)?.name} ${data.year}` : "No month of this period has ended yet";
  const ytd = data.cutoff_month > 0 && data.cutoff_month < data.months[data.months.length - 1].month;
  const c = data.coverage;
  const active = data.churches.filter((ch) => ch.active).length;
  const inactive = data.churches.length - active;
  const unknown = data.compliance_totals.churches_needing_review;
  // Coverage is only as complete as eligibility is known: say so rather than show "1 of 1".
  const coverage = c.expected === 0
    ? "no church-month reports are expected in the counted months"
    : `${c.expected_reported} of ${c.expected} expected church-month reports counted`;
  const outside = c.outside_reports > 0 ? ` (+${c.outside_reports} from months not required, included in totals)` : "";
  return (
    <div aria-label="Analytics scope" className="flex flex-col gap-1 text-sm">
      <p className="font-medium">
        {periodLabel(data.period)} {data.year} · {statusText} · {data.churches.length === data.church_options.length ? `All ${data.churches.length}` : `${data.churches.length} of ${data.church_options.length}`} churches
        {" "}({active} with an active church account, as on the Total Churches card{inactive > 0 ? `; ${inactive} inactive, listed for their reports or end month` : ""}) · {c.churches_reported} reported
      </p>
      <p className="text-muted-foreground">
        {counted}{ytd ? " (year to date: later months have not ended)" : ""} · {coverage}{outside}
        {data.default_start_month ? ` · expected from ${monthLabel(data.default_start_month)} unless a church has its own months` : " · no organization start month is configured"}
        {unknown > 0 && ` · ${unknown} ${unknown === 1 ? "church has" : "churches have"} no start month and ${unknown === 1 ? "is" : "are"} not counted as expected, so coverage is incomplete`}
        {data.comparison && ` · ${data.comparison.year}, same months: ${data.comparison.coverage.expected_reported} of ${data.comparison.coverage.expected}`}
      </p>
    </div>
  );
}

function Kpis({ data }: { data: AttendanceAnalytics }) {
  const prev = data.comparison;
  const cur = (key: string) => metricOf(data.metrics, key);
  const was = (key: string) => metricOf(prev?.metrics, key);
  const w = cur("worship_service");
  const legacy = cur("person_evangelized");
  const t = data.compliance_totals;
  const kpis = [
    { label: "Avg worship attendance per church-service", value: formatValue(w?.avg_per_church_service), note: `${formatValue(w?.total, "people", "No")} attendance entries ÷ ${w?.observations ?? 0} recorded church-services`, compare: prev && changeText(w?.avg_per_church_service, was("worship_service")?.avg_per_church_service, prev.year) },
    { label: "Network weekly worship attendance", value: formatValue(w?.network_weekly), note: `Average of ${w?.network_slots ?? 0} reporting-week slot totals; ${w?.network_slots_full ?? 0} had every expected church`, compare: prev && changeText(w?.network_weekly, was("worship_service")?.network_weekly, prev.year) },
    { label: "Tithes & Offering", value: formatValue(cur("tithes_offering")?.total, "php"), note: `Total from ${cur("tithes_offering")?.reports_with_value ?? 0} reports; ${cur("tithes_offering")?.churches_with_value ?? 0} churches with recorded offering`, compare: prev && changeText(cur("tithes_offering")?.total, was("tithes_offering")?.total, prev.year) },
    { label: "Persons Led to Christ", value: formatValue(cur("person_led_to_christ")?.total), note: legacy?.total != null ? `Not included: ${formatValue(legacy.total)} legacy "Persons Evangelized" (meaning unverified)` : "Total for the counted months", compare: prev && changeText(cur("person_led_to_christ")?.total, was("person_led_to_christ")?.total, prev.year) },
    { label: "Baptisms", value: formatValue(cur("baptism")?.total), note: "Total for the counted months", compare: prev && changeText(cur("baptism")?.total, was("baptism")?.total, prev.year) },
    { label: "Reports submitted", value: `${t.submitted} of ${t.expected}`, note: `${t.on_time} on time, ${t.late} late · ${t.awaiting} awaiting · ${t.overdue} overdue · ${t.not_filed} not filed` },
    { label: "Reports approved", value: `${t.approved} of ${t.expected}`, note: `${t.pending + t.needs_revision} still in review` },
  ];
  return (
    <section aria-label="Analytics summary" className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
      {kpis.map((kpi) => (
        <Card key={kpi.label} size="sm">
          <CardHeader><CardDescription>{kpi.label}</CardDescription><CardTitle className="font-mono text-2xl tabular-nums">{kpi.value}</CardTitle></CardHeader>
          <CardContent className="text-muted-foreground">{kpi.note}{kpi.compare && <p>{kpi.compare}</p>}</CardContent>
        </Card>
      ))}
      {prev && (
        <p className="text-sm text-muted-foreground sm:col-span-2 xl:col-span-4">
          Coverage differs between years ({data.coverage.expected_reported} of {data.coverage.expected} vs {prev.coverage.expected_reported} of {prev.coverage.expected} expected reports counted), so a change can reflect missing reports rather than ministry activity. Like-for-like {metricOf(data.metrics, data.metric)?.label}: {prev.like_for_like.churches} churches with data in both years, {formatPct(prev.like_for_like.change)}.
        </p>
      )}
    </section>
  );
}

function FullMetricsSummary({ data, statusText, onOpenChurch }: { data: AttendanceAnalytics; statusText: string; onOpenChurch: (id: number) => void }) {
  return (
    <section aria-labelledby="full-metrics-summary-heading" className="flex flex-col gap-3">
      <div>
        <h2 id="full-metrics-summary-heading" className="text-base font-medium">Full Metrics Summary</h2>
        <p className="text-sm text-muted-foreground">Every activity (row) for every church (column); church columns are totals for the counted months · {statusText}</p>
      </div>
      <div className="overflow-x-auto rounded-lg border">
        <Table aria-label="Full metrics summary" aria-describedby="full-metrics-note">
          <TableHeader>
            <TableRow>
              <TableHead scope="col" className="sticky left-0 z-10 min-w-32 bg-background sm:min-w-52">Metric</TableHead>
              <TableHead scope="col" className="text-right">Total</TableHead>
              <TableHead scope="col" className="text-right">Recorded entries</TableHead>
              <TableHead scope="col" className="text-right">Avg per church-service</TableHead>
              <TableHead scope="col" className="text-right">Network weekly avg</TableHead>
              {data.churches.map((c) => <TableHead scope="col" key={c.id} className="min-w-28 text-right"><button type="button" className="underline-offset-4 [text-transform:inherit] hover:underline" onClick={() => onOpenChurch(c.id)}>{c.name}</button></TableHead>)}
            </TableRow>
          </TableHeader>
          <TableBody>
            {data.metrics.flatMap((row, index) => {
              const cells = (
                <TableRow key={row.metric}>
                  <TableHead scope="row" className="sticky left-0 z-10 bg-background text-sm font-medium tracking-normal whitespace-normal text-foreground normal-case sm:whitespace-nowrap">{row.label}</TableHead>
                  <TableCell className="text-right tabular-nums">{formatValue(row.total, row.unit)}</TableCell>
                  <TableCell className="text-right tabular-nums">{row.observations}</TableCell>
                  <TableCell className="text-right tabular-nums">{row.unit === "people" ? formatValue(row.avg_per_church_service) : ""}</TableCell>
                  <TableCell className="text-right tabular-nums">{row.unit === "people" ? formatValue(row.network_weekly) : ""}</TableCell>
                  {row.churches.map((c) => (
                    <TableCell key={c.church_id} className={c.state === "value" ? "text-right tabular-nums" : "text-right text-muted-foreground"}>
                      {c.state === "value" ? formatValue(c.value, row.unit) : c.state === "blank" ? "—" : c.state === "not_approved" ? "Not approved" : "No report"}
                    </TableCell>
                  ))}
                </TableRow>
              );
              return row.group !== data.metrics[index - 1]?.group
                ? [<TableRow key={row.group} className="bg-muted/50 hover:bg-muted/50"><TableHead scope="colgroup" colSpan={5 + data.churches.length} className="text-xs font-semibold uppercase tracking-wide">{row.group}{row.unit === "as_reported" ? " · as reported (participants or events)" : ""}</TableHead></TableRow>, cells]
                : [cells];
            })}
          </TableBody>
        </Table>
      </div>
      <p id="full-metrics-note" className="text-sm text-muted-foreground">Attendance entries add up every recorded service and are not unique persons. Avg per church-service divides by the recorded services of that activity (0 included, blanks left out). — is a report that left the activity blank; Not approved is a report still in review; No report is no report in the counted months.</p>
    </section>
  );
}

function MonthlyTrend({ data }: { data: AttendanceAnalytics }) {
  const m = metricOf(data.metrics, data.metric);
  const unit = m?.unit;
  const prev = data.comparison;
  // A month with no value stays null: the line breaks there instead of dropping to 0.
  const points = data.trend.map((p, i) => ({ ...p, label: p.name.slice(0, 3), previous: prev?.trend[i]?.value ?? null }));
  const hasValues = points.some((p) => p.value != null || p.previous != null);
  const renderTooltip = ({ active, payload }: TooltipContentProps) => {
    if (!active || !payload?.length) return null;
    const p = payload[0]?.payload as (typeof points)[number] | undefined;
    if (!p) return null;
    return (
      <div className="rounded-md border bg-background px-3 py-2 text-sm shadow-sm">
        <p className="font-medium">{p.name}</p>
        <p>{data.year}: {p.counted ? formatValue(p.value, unit, "no data") : "not counted yet"}</p>
        {p.counted && <p className="text-muted-foreground">{p.churches_expected === 0 ? `No church owed this month; ${p.churches_reported} filed` : `${p.expected_reported} of ${p.churches_expected} expected churches reported${p.partial ? " (partial)" : ""}`}</p>}
        {prev && <p>{prev.year}: {formatValue(p.previous, unit, "no data")}</p>}
      </div>
    );
  };
  return (
    <>
    <div className="grid gap-6 xl:grid-cols-2">
      <Card>
        <CardHeader><CardTitle>Monthly trend</CardTitle><CardDescription>{m?.label} across the churches in scope · {periodLabel(data.period)} {data.year}. Gaps are months with no data; hollow points are partial months (fewer reports than expected).</CardDescription></CardHeader>
        <CardContent>
          {!hasValues ? <EmptyState icon={ChartColumn} title="No data available for this selection" description="Try another activity, period, church or reporting status." /> : (
            <div aria-label={`${m?.label} monthly trend chart`} role="img" className="h-72 w-full">
              <ResponsiveContainer width="100%" height="100%">
                <LineChart data={points} margin={{ top: 8, right: 8, left: 0, bottom: 8 }}>
                  <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" />
                  <XAxis dataKey="label" tick={{ fontSize: 11 }} />
                  <YAxis allowDecimals={false} />
                  <Tooltip content={renderTooltip} />
                  <Line type="linear" dataKey="value" name={String(data.year)} connectNulls={false} stroke="var(--primary)" strokeWidth={2} dot={PartialDot} />
                  {prev && <Line type="linear" dataKey="previous" name={String(prev.year)} connectNulls={false} stroke="var(--muted-foreground)" strokeDasharray="6 4" strokeWidth={2} dot={false} />}
                </LineChart>
              </ResponsiveContainer>
            </div>
          )}
        </CardContent>
      </Card>
      <ChurchBreakdown data={data} />
    </div>
      <Table aria-label="Monthly trend data">
        <TableCaption>Monthly {m?.label} totals with reporting coverage. A partial month had fewer reports than expected.</TableCaption>
        <TableHeader><TableRow><TableHead>Month</TableHead><TableHead className="text-right">{data.year}</TableHead><TableHead className="text-right">Entries</TableHead><TableHead className="text-right">Reported / expected</TableHead>{prev && <TableHead className="text-right">{prev.year}</TableHead>}</TableRow></TableHeader>
        <TableBody>
          {points.map((p) => (
            <TableRow key={p.month}>
              <TableCell>{p.name}</TableCell>
              <TableCell className="text-right tabular-nums">{p.counted ? formatValue(p.value, unit, "No data") : "Not yet due"}</TableCell>
              <TableCell className="text-right tabular-nums">{p.counted ? p.observations : ""}</TableCell>
              <TableCell className="text-right tabular-nums">{!p.counted ? "" : p.churches_expected === 0 ? (p.churches_reported ? `not required (${p.churches_reported} filed)` : "not required") : `${p.expected_reported} / ${p.churches_expected}${p.partial ? " (partial)" : ""}${p.churches_reported > p.expected_reported ? ` +${p.churches_reported - p.expected_reported} not required` : ""}`}</TableCell>
              {prev && <TableCell className="text-right tabular-nums">{p.counted ? formatValue(p.previous, unit, "No data") : ""}</TableCell>}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </>
  );
}

// ChurchBreakdown is the selected activity's total per church as horizontal bars.
// Every church in scope is a row; a church with no data has no bar, never a 0 bar.
function ChurchBreakdown({ data }: { data: AttendanceAnalytics }) {
  const m = metricOf(data.metrics, data.metric);
  const prev = data.comparison;
  const rows = data.church_rows.map((r) => ({ church_name: r.church_name, value: r.total, previous: r.previous_total ?? null, missing: missingText(data, r.church_id) }));
  const noData = rows.filter((r) => r.value == null && r.missing === "No data").map((r) => r.church_name);
  const inReview = rows.filter((r) => r.value == null && r.missing !== "No data").map((r) => `${r.church_name} (${r.missing.toLowerCase()})`);
  const renderTooltip = ({ active, payload }: TooltipContentProps) => {
    if (!active || !payload?.length) return null;
    const r = payload[0]?.payload as (typeof rows)[number] | undefined;
    if (!r) return null;
    return (
      <div className="rounded-md border bg-background px-3 py-2 text-sm shadow-sm">
        <p className="font-medium">{r.church_name}</p>
        <p>{data.year}: {formatValue(r.value, m?.unit, r.missing.toLowerCase())}</p>
        {prev && <p>{prev.year}: {formatValue(r.previous, m?.unit, "no data")}</p>}
      </div>
    );
  };
  return (
    <Card>
      <CardHeader><CardTitle>Church breakdown</CardTitle><CardDescription>Total {m?.label} by church · counted months of {periodLabel(data.period)} {data.year}</CardDescription></CardHeader>
      <CardContent>
        {rows.every((r) => r.value == null && r.previous == null) ? <EmptyState icon={ChartColumn} title="No data available for this selection" description="Try another activity, period, church or reporting status." /> : (
          <>
            <div aria-label={`${m?.label} church breakdown chart`} role="img" className="w-full" style={{ height: Math.max(288, rows.length * (prev ? 34 : 26) + 48) }}>
              <ResponsiveContainer width="100%" height="100%">
                <BarChart data={rows} layout="vertical" margin={{ top: 8, right: 8, left: 8, bottom: 8 }}>
                  <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" />
                  <XAxis type="number" allowDecimals={false} />
                  <YAxis dataKey="church_name" type="category" width={130} interval={0} tick={{ fontSize: 11 }} />
                  <Tooltip content={renderTooltip} />
                  <Bar dataKey="value" name={String(data.year)} fill="var(--primary)" />
                  {prev && <Bar dataKey="previous" name={String(prev.year)} fill="var(--muted-foreground)" />}
                </BarChart>
              </ResponsiveContainer>
            </div>
            {noData.length > 0 && <p className="mt-2 text-xs text-muted-foreground">No data for {data.year}: {noData.join(", ")}.</p>}
            {inReview.length > 0 && <p className="mt-2 text-xs text-muted-foreground">Submitted but not yet approved: {inReview.join(", ")}.</p>}
          </>
        )}
      </CardContent>
    </Card>
  );
}

// PartialDot draws a month from fewer reports than expected as a hollow point.
function PartialDot({ cx, cy, payload }: { cx?: number; cy?: number; payload?: { partial?: boolean; month?: number } }) {
  if (cx == null || cy == null) return null;
  return <circle key={payload?.month} cx={cx} cy={cy} r={3.5} stroke="var(--primary)" strokeWidth={2} fill={payload?.partial ? "var(--background)" : "var(--primary)"} />;
}

type SortKey = "church_name" | "total" | "average" | "change" | "tithes_total" | "completeness";

function ChurchComparison({ data, onOpenChurch }: { data: AttendanceAnalytics; onOpenChurch: (id: number) => void }) {
  const [sort, setSort] = useState<{ key: SortKey; desc: boolean }>({ key: "church_name", desc: false });
  const m = metricOf(data.metrics, data.metric);
  const att = m?.unit === "people";
  const prev = data.comparison;
  const value = (r: ChurchMetric) => r[sort.key] ?? null;
  // Missing values sort last either way, so a church with no data is never hidden or mistaken for 0.
  const rows = [...data.church_rows].sort((a, b) => {
    const x = value(a), y = value(b);
    if (x == null || y == null) return x == null ? (y == null ? 0 : 1) : -1;
    const c = typeof x === "string" ? x.localeCompare(String(y)) : x - (y as number);
    return sort.desc ? -c : c;
  });
  const header = (key: SortKey, label: string, right = true) => (
    <TableHead scope="col" className={right ? "text-right" : undefined} aria-sort={sort.key === key ? (sort.desc ? "descending" : "ascending") : "none"}>
      <button type="button" className="font-medium [text-transform:inherit]" onClick={() => setSort({ key, desc: sort.key === key ? !sort.desc : key !== "church_name" })}>{label}{sort.key === key ? (sort.desc ? " ↓" : " ↑") : ""}</button>
    </TableHead>
  );
  return (
    <div className="flex flex-col gap-2">
      <h3 className="text-sm font-medium">Church comparison · {m?.label}</h3>
      <div className="overflow-x-auto rounded-lg border">
        <Table aria-label="Church comparison" aria-describedby="church-comparison-note">
          <TableHeader>
            <TableRow>
              {header("church_name", "Church", false)}
              {header("total", "Total")}
              <TableHead scope="col" className="text-right">Recorded entries</TableHead>
              {att && header("average", "Average per service")}
              {prev && <TableHead scope="col" className="text-right">{prev.year} {att ? "average" : "total"}</TableHead>}
              {prev && header("change", "Change")}
              {header("tithes_total", "Tithes & Offering")}
              <TableHead scope="col" className="text-right">Submitted / expected</TableHead>
              <TableHead scope="col" className="text-right">Approved</TableHead>
              {header("completeness", "Submission completeness")}
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((r) => (
              <TableRow key={r.church_id}>
                <TableCell><button type="button" className="text-left underline underline-offset-4" onClick={() => onOpenChurch(r.church_id)}>{r.church_name}</button></TableCell>
                <TableCell className="text-right tabular-nums">{formatValue(r.total, m?.unit, missingText(data, r.church_id))}</TableCell>
                <TableCell className="text-right tabular-nums">{r.observations}</TableCell>
                {att && <TableCell className="text-right tabular-nums">{formatValue(r.average)}</TableCell>}
                {prev && <TableCell className="text-right tabular-nums">{formatValue(att ? r.previous_average : r.previous_total, m?.unit, "No data")}</TableCell>}
                {prev && <TableCell className="text-right tabular-nums">{formatPct(r.change)}</TableCell>}
                <TableCell className="text-right tabular-nums">{formatValue(r.tithes_total, "php")}</TableCell>
                <TableCell className="text-right tabular-nums">{r.submitted} / {r.expected}</TableCell>
                <TableCell className="text-right tabular-nums">{r.approved}</TableCell>
                <TableCell className="text-right tabular-nums">{r.completeness == null ? "—" : `${Math.round(r.completeness * 100)}%`}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
      <p id="church-comparison-note" className="text-sm text-muted-foreground">One row per church in scope, including churches with no data. Change is shown only when both periods have data and the previous is above 0.</p>
    </div>
  );
}

function WeekSlots({ data }: { data: AttendanceAnalytics }) {
  const m = metricOf(data.metrics, data.metric);
  const slots = data.slots.filter((s) => s.total != null || s.churches_reported > 0);
  if (slots.length === 0) return null;
  return (
    <div className="flex flex-col gap-2">
      <h3 className="text-sm font-medium">Network totals by reporting-week slot · {m?.label}</h3>
      <div className="max-h-96 overflow-auto rounded-lg border">
        <Table aria-label="Reporting-week slots">
          <TableCaption>Week 1–5 are the report form&apos;s slots, not calendar dates, and need not be the same dates for every church.</TableCaption>
          <TableHeader><TableRow><TableHead>Month</TableHead><TableHead>Slot</TableHead><TableHead className="text-right">Network total</TableHead><TableHead className="text-right">Churches with a value / reported</TableHead></TableRow></TableHeader>
          <TableBody>
            {slots.map((s) => (
              <TableRow key={`${s.month}-${s.week}`}>
                <TableCell>{data.months.find((x) => x.month === s.month)?.name}</TableCell>
                <TableCell>Week {s.week}</TableCell>
                <TableCell className="text-right tabular-nums">{formatValue(s.total, m?.unit, "No data")}</TableCell>
                <TableCell className="text-right tabular-nums">{s.churches_with_value} / {s.churches_reported}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
    </div>
  );
}
