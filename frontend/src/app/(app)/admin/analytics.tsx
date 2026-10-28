"use client";

import { useMutation, useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { toast } from "sonner";
import { ChartColumn, Download } from "lucide-react";
import { Bar, BarChart, CartesianGrid, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import type { TooltipContentProps } from "recharts";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { NativeSelect } from "@/components/ui/native-select";
import { Table, TableBody, TableCaption, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { ErrorRetry } from "@/components/error-retry";
import { ANALYTICS_PERIODS, exportAttendanceAnalytics, getAttendanceAnalytics, type AnalyticsPeriod, type AttendanceStatus } from "@/lib/analytics";
import { METRICS } from "@/lib/report-form";
import { FIRST_YEAR } from "@/lib/reports";
import { saveFile } from "@/lib/api";
import { errorText } from "@/lib/auth-form";
import { useSession } from "@/lib/session";
import { getStats } from "@/lib/stats";
import { EmptyState } from "@/components/empty-state";
import { AnalyticsSkeleton } from "@/components/loading-skeletons";

const formatValue = (value: number) => value.toLocaleString(undefined, { maximumFractionDigits: 2 });
const formatSummaryValue = (metric: string, value: number | null | undefined) => {
  if (value == null) return "—";
  if (metric === "tithes_offering") {
    return new Intl.NumberFormat("en-PH", { style: "currency", currency: "PHP", minimumFractionDigits: 2, maximumFractionDigits: 2 }).format(value);
  }
  return formatValue(value);
};
const periodLabel = (period: AnalyticsPeriod) => ANALYTICS_PERIODS.find((item) => item.value === period)?.label ?? period;
const changeText = (current: number, previous?: number) => {
  if (previous === undefined) return "";
  if (previous === 0) return current === 0 ? "0% vs last year" : "New vs last year";
  const change = ((current - previous) / previous) * 100;
  return `${change >= 0 ? "+" : ""}${formatValue(change)}% vs last year`;
};

export function Analytics() {
  const { can } = useSession();
  const allowed = can("tracker:read");
  const [year, setYear] = useState(() => new Date().getFullYear());
  const [period, setPeriod] = useState<AnalyticsPeriod>("full_year");
  const [metric, setMetric] = useState("worship_service");
  const [status, setStatus] = useState<AttendanceStatus>("approved");
  const [compare, setCompare] = useState(false);
  const { data, error, isPending, refetch } = useQuery({
    queryKey: ["analytics-attendance", year, metric, status, period, compare],
    queryFn: () => getAttendanceAnalytics(year, metric, status, period, compare),
    enabled: allowed,
    staleTime: 5 * 60_000, // the charts change only when a report does, and refreshAdminViews invalidates them then
  });
  const { data: stats } = useQuery({
    queryKey: ["admin-stats"],
    queryFn: getStats,
    enabled: allowed,
  });

  const exportExcel = useMutation({
    mutationFn: () => exportAttendanceAnalytics(year, metric, status, period, compare),
    onSuccess: (file) => {
      saveFile(file);
      toast.success(`Downloaded ${file.filename}`);
    },
    onError: (err) => toast.error(errorText(err, "Could not download the file. Try again.")),
  });

  if (!allowed) return <p className="text-sm text-muted-foreground">You are not allowed to view analytics.</p>;

  const currentYear = new Date().getFullYear();
  const years = Array.from({ length: Math.max(1, currentYear - FIRST_YEAR + 1) }, (_, i) => FIRST_YEAR + i);
  const trend = data?.trend ?? [];
  const comparison = data?.comparison;
  // The weekly points add up to months: a full year reads as 12 points, not 60 mostly empty weeks.
  const byMonth = (points: { month: string; value: number }[] = []) => points.reduce<Map<string, number>>((totals, point) => totals.set(point.month, (totals.get(point.month) ?? 0) + point.value), new Map());
  const monthlyTotals = byMonth(trend);
  const previousMonthlyTotals = comparison ? byMonth(comparison.trend) : undefined;
  const trendData = [...monthlyTotals].map(([month, value]) => ({ month, label: month.slice(0, 3), value, previousValue: previousMonthlyTotals?.get(month) ?? 0 }));
  const churches = data?.churches ?? [];
  const comparisonChurches = comparison?.churches ?? [];
  const churchNames = [...new Set([...churches.map((church) => church.church_name), ...comparisonChurches.map((church) => church.church_name)])];
  const currentChurches = new Map(churches.map((church) => [church.church_name, church.value]));
  const previousChurches = new Map(comparisonChurches.map((church) => [church.church_name, church.value]));
  const churchData = churchNames.map((church_name) => ({ church_name, value: currentChurches.get(church_name) ?? 0, previousValue: previousChurches.get(church_name) ?? 0 }));
  const summary = data?.summary ?? [];
  const summaryChurchNames = [...new Set(summary.flatMap((row) => row.churches.map((church) => church.church_name)))];
  const noReportChurches = data?.no_report_churches ?? [];
  const churchesInScope = summaryChurchNames.length + noReportChurches.length;
  const weeks = data?.weeks ?? 0;
  const summaryTotal = (key: string) => summary.find((row) => row.metric === key)?.total ?? null;
  const perWeek = (total: number | null | undefined) => total == null || weeks === 0 ? null : total / weeks;
  const statusText = status === "approved" ? "Approved reports" : "All submitted reports";
  const metricLabel = METRICS.find((item) => item.key === metric)?.label ?? metric;
  const hasTrendValues = trendData.some((point) => point.value !== 0 || point.previousValue !== 0);
  const hasChurchValues = churchData.some((church) => church.value !== 0 || church.previousValue !== 0);
  const yearlyTotal = trend.reduce((total, point) => total + point.value, 0);
  const peakMonth = [...monthlyTotals].reduce<[string, number]>((peak, current) => current[1] > peak[1] ? current : peak, ["", 0]);
  const previousTotal = comparison?.trend.reduce((total, point) => total + point.value, 0);
  const previousPeakMonth = previousMonthlyTotals
    ? [...previousMonthlyTotals].reduce<[string, number]>((peak, current) => current[1] > peak[1] ? current : peak, ["", 0])
    : undefined;
  const activeChurches = churches.filter((church) => church.value > 0).length;
  const previousActiveChurches = comparisonChurches.filter((church) => church.value > 0).length;
  const renderTooltip = ({ active, payload }: TooltipContentProps) => {
    if (!active || !payload?.length) return null;
    const point = payload[0]?.payload as { month?: string; church_name?: string } | undefined;
    if (!point) return null;
    const label = point.month ?? point.church_name;
    return <div className="rounded-md border bg-background px-3 py-2 text-sm shadow-sm"><p>{label}</p>{payload.map((item) => <p key={String(item.dataKey)}>{item.name}: {formatValue(Number(item.value))}</p>)}</div>;
  };
  const kpis = [
    { label: "Avg weekly worship attendance", value: formatSummaryValue("worship_service", perWeek(summaryTotal("worship_service"))), note: `Worship Service total ÷ ${weeks} weeks with reports` },
    { label: "Tithes & Offering", value: formatSummaryValue("tithes_offering", summaryTotal("tithes_offering")), note: "Total for the period" },
    { label: "Baptisms", value: formatSummaryValue("baptism", summaryTotal("baptism")), note: "Total for the period" },
    { label: "Persons evangelized", value: formatSummaryValue("person_evangelized", summaryTotal("person_evangelized")), note: "Total for the period" },
    { label: "Churches reporting", value: `${summaryChurchNames.length} of ${churchesInScope}`, note: noReportChurches.length ? `${noReportChurches.length} with no report` : "Every church reported" },
  ];
  const analyticsSummary = <section aria-label="Analytics summary" className="grid gap-4 sm:grid-cols-2 xl:grid-cols-5">
    {kpis.map((kpi) => <Card key={kpi.label} size="sm"><CardHeader><CardDescription>{kpi.label}</CardDescription><CardTitle className="font-mono text-2xl tabular-nums">{kpi.value}</CardTitle></CardHeader><CardContent className="text-muted-foreground">{kpi.note}</CardContent></Card>)}
  </section>;
  const trendSummary = <section aria-label="Trend summary" className="grid gap-4 md:grid-cols-3">
    <Card size="sm"><CardHeader><CardDescription>Total {metricLabel}</CardDescription><CardTitle className="font-mono text-2xl tabular-nums">{formatSummaryValue(metric, yearlyTotal)}</CardTitle></CardHeader><CardContent className="text-muted-foreground">{periodLabel(period)} {year}{comparison && <p>{changeText(yearlyTotal, previousTotal)} · vs {comparison.year}</p>}</CardContent></Card>
    <Card size="sm"><CardHeader><CardDescription>Peak Month</CardDescription><CardTitle className="text-2xl">{peakMonth[0] || "—"}</CardTitle></CardHeader><CardContent className="text-muted-foreground">{peakMonth[0] ? `${formatSummaryValue(metric, peakMonth[1])} ${metricLabel.toLowerCase()}` : "No data available"}{comparison && <p>{changeText(peakMonth[1], previousPeakMonth?.[1])}</p>}</CardContent></Card>
    <Card size="sm"><CardHeader><CardDescription>Active Churches</CardDescription><CardTitle className="text-2xl">{activeChurches} out of {stats?.churches_total ?? "—"} churches</CardTitle></CardHeader><CardContent className="text-muted-foreground">Reported a value greater than 0{comparison && <p>{changeText(activeChurches, previousActiveChurches)}</p>}</CardContent></Card>
  </section>;
  const columnCount = 3 + summaryChurchNames.length + noReportChurches.length;
  const fullMetricsSummary = <section aria-labelledby="full-metrics-summary-heading" className="flex flex-col gap-3"><div><h2 id="full-metrics-summary-heading" className="text-base font-medium">Full Metrics Summary</h2><p className="text-sm text-muted-foreground">Every activity (row) for every church (column) · {periodLabel(period)} {year} · {statusText}</p></div><div className="overflow-x-auto rounded-lg border"><Table aria-label="Full metrics summary"><TableCaption>Attendance totals add up every week&apos;s count; Avg / Week divides by the weeks with reports, so it is the typical week. — means the church reported but left the activity blank; No report means no report for the period.</TableCaption><TableHeader><TableRow><TableHead scope="col" className="sticky left-0 bg-background">Metric Name</TableHead><TableHead scope="col" className="text-right">Total for Period</TableHead><TableHead scope="col" className="text-right">Avg / Week</TableHead>{summaryChurchNames.map((church) => <TableHead scope="col" key={church} className="text-right">{church}</TableHead>)}{noReportChurches.map((church) => <TableHead scope="col" key={church} className="text-right text-muted-foreground">{church}</TableHead>)}</TableRow></TableHeader><TableBody>{summary.flatMap((row, index) => { const label = METRICS.find((item) => item.key === row.metric)?.label ?? row.metric; const values = new Map(row.churches.map((church) => [church.church_name, church.value])); const attendance = row.group === "Attendance"; const cells = <TableRow key={row.metric}><TableHead scope="row" className="sticky left-0 bg-background text-sm font-medium tracking-normal text-foreground normal-case">{label}</TableHead><TableCell className="text-right tabular-nums">{formatSummaryValue(row.metric, row.total)}</TableCell><TableCell className="text-right tabular-nums">{attendance ? formatSummaryValue(row.metric, perWeek(row.total)) : ""}</TableCell>{summaryChurchNames.map((church) => <TableCell key={church} className="text-right tabular-nums">{formatSummaryValue(row.metric, values.get(church))}</TableCell>)}{noReportChurches.map((church) => <TableCell key={church} className="text-right text-muted-foreground">No report</TableCell>)}</TableRow>; return row.group && row.group !== summary[index - 1]?.group ? [<TableRow key={row.group} className="bg-muted/50 hover:bg-muted/50"><TableHead scope="colgroup" colSpan={columnCount} className="text-xs font-semibold uppercase tracking-wide">{row.group}</TableHead></TableRow>, cells] : [cells]; })}</TableBody></Table></div></section>;

  return (
    <div className="flex flex-col gap-6">
      <header className="flex flex-wrap items-center justify-between gap-4"><div><h2 className="text-xl font-medium tracking-tight">Analytics</h2><p className="mt-1 text-sm text-muted-foreground">Every church and every activity for the period, then the trend of one activity.</p></div><div className="flex flex-wrap gap-2"><Button type="button" variant="outline" onClick={() => exportExcel.mutate()} disabled={!data || exportExcel.isPending} aria-busy={exportExcel.isPending}><Download aria-hidden="true" />{exportExcel.isPending ? "Downloading…" : "Export to Excel"}</Button></div></header>
      <form onSubmit={(event) => event.preventDefault()} className="grid gap-4 sm:grid-cols-3">
        <div className="flex flex-col gap-2"><Label htmlFor="analytics-year">Year</Label><NativeSelect id="analytics-year" value={year} onChange={(event) => setYear(Number(event.target.value))}>{years.map((option) => <option key={option} value={option}>{option}</option>)}</NativeSelect></div>
        <div className="flex flex-col gap-2"><Label htmlFor="analytics-period">Period</Label><NativeSelect id="analytics-period" value={period} onChange={(event) => setPeriod(event.target.value as AnalyticsPeriod)}>{ANALYTICS_PERIODS.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}</NativeSelect></div>
        <div className="flex flex-col gap-2"><Label htmlFor="analytics-status">Status</Label><NativeSelect id="analytics-status" value={status} onChange={(event) => setStatus(event.target.value as AttendanceStatus)}><option value="approved">Approved</option><option value="all">All Submitted</option></NativeSelect></div>
        <label htmlFor="analytics-compare" className="flex min-h-11 items-center gap-3 text-sm sm:col-span-3"><input id="analytics-compare" type="checkbox" className="size-5 accent-[var(--primary)]" checked={compare} onChange={(event) => setCompare(event.target.checked)} /><span>Compare with previous year</span></label>
      </form>
      {isPending ? <AnalyticsSkeleton /> : error ? <ErrorRetry message={error.message} onRetry={() => refetch()} /> : (
        <>
          <p aria-label="Analytics scope" className="text-sm font-medium">{periodLabel(period)} {year} · {statusText} · {summaryChurchNames.length} of {churchesInScope} churches reporting</p>
          {analyticsSummary}
          {fullMetricsSummary}
          <section aria-labelledby="analytics-trend-heading" className="flex flex-col gap-4 border-t pt-6">
            <div className="flex flex-wrap items-end justify-between gap-4"><div><h2 id="analytics-trend-heading" className="text-base font-medium">Trend</h2><p className="text-sm text-muted-foreground">Pick one activity to see how it moves month by month and church by church.</p></div><div className="flex min-w-56 flex-col gap-2"><Label htmlFor="analytics-metric">Trend for</Label><NativeSelect id="analytics-metric" value={metric} onChange={(event) => setMetric(event.target.value)}>{METRICS.map((option) => <option key={option.key} value={option.key}>{option.label}</option>)}</NativeSelect></div></div>
            {trendSummary}
            <div className="grid gap-6 xl:grid-cols-2">
              <Card><CardHeader><CardTitle>Monthly trend</CardTitle><CardDescription>{metricLabel} across all churches · {periodLabel(period)} {year}</CardDescription></CardHeader><CardContent>{!hasTrendValues ? <EmptyState icon={ChartColumn} title="No data available for this selection" description="Try another metric, period, or reporting status." /> : <div aria-label={`${metricLabel} monthly trend chart`} role="img" className="h-72 w-full"><ResponsiveContainer width="100%" height="100%"><LineChart data={trendData} margin={{ top: 8, right: 8, left: 0, bottom: 8 }}><CartesianGrid strokeDasharray="3 3" stroke="var(--border)" /><XAxis dataKey="label" tick={{ fontSize: 11 }} /><YAxis allowDecimals={false} label={{ value: metricLabel, angle: -90, position: "insideLeft" }} /><Tooltip content={renderTooltip} /><Line type="monotone" dataKey="value" name={`${year} ${metricLabel}`} stroke="var(--primary)" strokeWidth={2} dot={{ r: 3, fill: "var(--primary)" }} />{comparison && <Line type="monotone" dataKey="previousValue" name={`${comparison.year} ${metricLabel}`} stroke="var(--muted-foreground)" strokeDasharray="6 4" strokeWidth={2} dot={false} />}</LineChart></ResponsiveContainer></div>}</CardContent></Card>
              <Card><CardHeader><CardTitle>Church breakdown</CardTitle><CardDescription>Total {metricLabel} reported by church · {periodLabel(period)} {year}</CardDescription></CardHeader><CardContent>{!hasChurchValues ? <EmptyState icon={ChartColumn} title="No data available for this selection" description="Try another metric, period, or reporting status." /> : <div aria-label={`${metricLabel} church breakdown chart`} role="img" className="h-72 w-full"><ResponsiveContainer width="100%" height="100%"><BarChart data={churchData} layout="vertical" margin={{ top: 8, right: 8, left: 8, bottom: 8 }}><CartesianGrid strokeDasharray="3 3" stroke="var(--border)" /><XAxis type="number" allowDecimals={false} label={{ value: metricLabel, position: "insideBottom", offset: -4 }} /><YAxis dataKey="church_name" type="category" width={100} tick={{ fontSize: 11 }} /><Tooltip content={renderTooltip} /><Bar dataKey="value" name={`${year} ${metricLabel}`} fill="var(--primary)" />{comparison && <Bar dataKey="previousValue" name={`${comparison.year} ${metricLabel}`} fill="var(--muted-foreground)" />}</BarChart></ResponsiveContainer></div>}</CardContent></Card>
            </div>
            <div className="grid gap-6 xl:grid-cols-2">
              <Table aria-label="Monthly trend data"><TableCaption>Monthly {metricLabel} totals for {periodLabel(period)} {year}.</TableCaption><TableHeader><TableRow><TableHead>Month</TableHead><TableHead className="text-right">{year}</TableHead>{comparison && <TableHead className="text-right">{comparison.year}</TableHead>}</TableRow></TableHeader><TableBody>{trendData.map((point) => <TableRow key={point.month}><TableCell>{point.month}</TableCell><TableCell className="text-right tabular-nums">{formatSummaryValue(metric, point.value)}</TableCell>{comparison && <TableCell className="text-right tabular-nums">{formatSummaryValue(metric, point.previousValue)}</TableCell>}</TableRow>)}</TableBody></Table>
              <Table aria-label="Church breakdown data"><TableCaption>Total {metricLabel} by church for {periodLabel(period)} {year}.</TableCaption><TableHeader><TableRow><TableHead>Church</TableHead><TableHead className="text-right">{year}</TableHead>{comparison && <TableHead className="text-right">{comparison.year}</TableHead>}</TableRow></TableHeader><TableBody>{churchData.map((church) => <TableRow key={church.church_name}><TableCell>{church.church_name}</TableCell><TableCell className="text-right tabular-nums">{formatSummaryValue(metric, church.value)}</TableCell>{comparison && <TableCell className="text-right tabular-nums">{formatSummaryValue(metric, church.previousValue)}</TableCell>}</TableRow>)}</TableBody></Table>
            </div>
          </section>
        </>
      )}
    </div>
  );
}
