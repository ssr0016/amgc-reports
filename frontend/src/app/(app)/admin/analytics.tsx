"use client";

import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { Download } from "lucide-react";
import { Bar, BarChart, CartesianGrid, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import type { TooltipContentProps } from "recharts";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { NativeSelect } from "@/components/ui/native-select";
import { Table, TableBody, TableCaption, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { ErrorRetry } from "@/components/error-retry";
import { Loading } from "@/components/loading";
import { ANALYTICS_PERIODS, getAttendanceAnalytics, type AnalyticsPeriod, type AttendanceAnalytics, type AttendanceStatus } from "@/lib/analytics";
import { METRICS } from "@/lib/report-form";
import { FIRST_YEAR } from "@/lib/reports";
import { useSession } from "@/lib/session";
import { getStats } from "@/lib/stats";

const formatValue = (value: number) => value.toLocaleString(undefined, { maximumFractionDigits: 2 });
const periodLabel = (period: AnalyticsPeriod) => ANALYTICS_PERIODS.find((item) => item.value === period)?.label ?? period;
const changeText = (current: number, previous?: number) => {
  if (previous === undefined) return "";
  if (previous === 0) return current === 0 ? "0% vs last year" : "New vs last year";
  const change = ((current - previous) / previous) * 100;
  return `${change >= 0 ? "+" : ""}${formatValue(change)}% vs last year`;
};

type CsvInput = {
  data: AttendanceAnalytics;
  metricLabel: string;
  yearlyTotal: number;
  peakMonth: [string, number];
  activeChurches: number;
  totalChurches: number;
};

const csvCell = (value: unknown) => {
  const text = value == null ? "" : String(value);
  return /[",\r\n]/.test(text) ? `"${text.replaceAll('"', '""')}"` : text;
};

export function buildAnalyticsCsv({ data, metricLabel, yearlyTotal, peakMonth, activeChurches, totalChurches }: CsvInput) {
  const comparison = data.comparison;
  const previousTotal = comparison?.trend.reduce((total, point) => total + point.value, 0);
  const previousMonthlyTotals = comparison?.trend.reduce<Record<string, number>>((totals, point) => {
    totals[point.month] = (totals[point.month] ?? 0) + point.value;
    return totals;
  }, {});
  const previousPeakMonth = previousMonthlyTotals
    ? Object.entries(previousMonthlyTotals).reduce<[string, number]>((peak, current) => current[1] > peak[1] ? current : peak, ["", 0])
    : ["", 0] as [string, number];
  const previousActiveChurches = comparison?.churches.filter((church) => church.value > 0).length;
  const previousYear = comparison?.year;
  const previousTrend = new Map(comparison?.trend.map((point) => [`${point.month}-${point.week}`, point.value]));
  const churchNames = [...new Set([...data.churches.map((church) => church.church_name), ...(comparison?.churches.map((church) => church.church_name) ?? [])])];
  const currentChurches = new Map(data.churches.map((church) => [church.church_name, church.value]));
  const previousChurches = new Map(comparison?.churches.map((church) => [church.church_name, church.value]));
  const rows: unknown[][] = [
    ["Analytics", `${metricLabel} · ${data.year} · ${periodLabel(data.period as AnalyticsPeriod)}`],
    [],
    ["KPI", "Value", ...(previousYear ? [String(previousYear), "Change"] : [])],
    ["Total", yearlyTotal, ...(previousYear ? [previousTotal, changeText(yearlyTotal, previousTotal)] : [])],
    ["Peak Month", peakMonth[0], ...(previousYear ? [previousPeakMonth[0], changeText(peakMonth[1], previousPeakMonth[1])] : [])],
    ["Active Churches", `${activeChurches} out of ${totalChurches}`, ...(previousYear ? [previousActiveChurches, changeText(activeChurches, previousActiveChurches)] : [])],
    [],
    ["Weekly trend"],
    ["Month", "Week", String(data.year), ...(previousYear ? [String(previousYear)] : [])],
    ...data.trend.filter((point) => point.value !== 0 || (previousTrend.get(`${point.month}-${point.week}`) ?? 0) !== 0).map((point) => [point.month, point.week, point.value, ...(previousYear ? [previousTrend.get(`${point.month}-${point.week}`) ?? 0] : [])]),
    [],
    ["Church breakdown"],
    ["Church", String(data.year), ...(previousYear ? [String(previousYear)] : [])],
    ...churchNames.map((name) => [name, currentChurches.get(name) ?? 0, ...(previousYear ? [previousChurches.get(name) ?? 0] : [])]),
  ];
  return `${rows.map((row) => row.map(csvCell).join(",")).join("\r\n")}\r\n`;
}

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
    staleTime: 30_000,
  });
  const { data: stats } = useQuery({
    queryKey: ["admin-stats"],
    queryFn: getStats,
    enabled: allowed,
    staleTime: 30_000,
  });

  if (!allowed) return <p className="text-sm text-muted-foreground">You are not allowed to view analytics.</p>;

  const currentYear = new Date().getFullYear();
  const years = Array.from({ length: Math.max(1, currentYear - FIRST_YEAR + 1) }, (_, i) => FIRST_YEAR + i);
  const trend = data?.trend ?? [];
  const comparison = data?.comparison;
  const previousTrend = new Map(comparison?.trend.map((point) => [`${point.month}-${point.week}`, point.value]));
  const trendData = trend.map((point) => ({ ...point, label: `${point.month.slice(0, 3)} W${point.week}`, previousValue: previousTrend.get(`${point.month}-${point.week}`) ?? 0 }));
  const trendWithData = trendData.filter((point) => point.value !== 0 || point.previousValue !== 0);
  const churches = data?.churches ?? [];
  const comparisonChurches = comparison?.churches ?? [];
  const churchNames = [...new Set([...churches.map((church) => church.church_name), ...comparisonChurches.map((church) => church.church_name)])];
  const currentChurches = new Map(churches.map((church) => [church.church_name, church.value]));
  const previousChurches = new Map(comparisonChurches.map((church) => [church.church_name, church.value]));
  const churchData = churchNames.map((church_name) => ({ church_name, value: currentChurches.get(church_name) ?? 0, previousValue: previousChurches.get(church_name) ?? 0 }));
  const metricLabel = METRICS.find((item) => item.key === metric)?.label ?? metric;
  const hasTrendValues = trendData.some((point) => point.value !== 0 || point.previousValue !== 0);
  const hasChurchValues = churchData.some((church) => church.value !== 0 || church.previousValue !== 0);
  const yearlyTotal = trend.reduce((total, point) => total + point.value, 0);
  const monthlyTotals = trend.reduce<Record<string, number>>((totals, point) => {
    totals[point.month] = (totals[point.month] ?? 0) + point.value;
    return totals;
  }, {});
  const peakMonth = Object.entries(monthlyTotals).reduce<[string, number]>((peak, current) => current[1] > peak[1] ? current : peak, ["", 0]);
  const previousTotal = comparison?.trend.reduce((total, point) => total + point.value, 0);
  const previousMonthlyTotals = comparison?.trend.reduce<Record<string, number>>((totals, point) => {
    totals[point.month] = (totals[point.month] ?? 0) + point.value;
    return totals;
  }, {});
  const previousPeakMonth = previousMonthlyTotals
    ? Object.entries(previousMonthlyTotals).reduce<[string, number]>((peak, current) => current[1] > peak[1] ? current : peak, ["", 0])
    : undefined;
  const activeChurches = churches.filter((church) => church.value > 0).length;
  const previousActiveChurches = comparisonChurches.filter((church) => church.value > 0).length;
  const renderTooltip = ({ active, payload }: TooltipContentProps) => {
    if (!active || !payload?.length) return null;
    const point = payload[0]?.payload as { month?: string; week?: number; church_name?: string } | undefined;
    if (!point) return null;
    const label = point.month ? `${point.month}, Week ${point.week}` : point.church_name;
    return <div className="rounded-md border bg-background px-3 py-2 text-sm shadow-sm"><p>{label}</p>{payload.map((item) => <p key={String(item.dataKey)}>{item.name}: {formatValue(Number(item.value))}</p>)}</div>;
  };
  const exportCsv = () => {
    if (!data) return;
    const csv = buildAnalyticsCsv({ data, metricLabel, yearlyTotal, peakMonth, activeChurches, totalChurches: stats?.churches_total ?? 0 });
    const url = URL.createObjectURL(new Blob([csv], { type: "text/csv;charset=utf-8" }));
    const link = document.createElement("a");
    link.href = url;
    link.download = `analytics-${year}-${period}.csv`;
    link.click();
    URL.revokeObjectURL(url);
  };

  return (
    <div className="flex flex-col gap-6">
      <header className="flex flex-wrap items-center justify-between gap-4"><div><h2 className="text-xl font-medium tracking-tight">Analytics</h2><p className="mt-1 text-sm text-muted-foreground">Attendance trends and church totals.</p></div><Button type="button" variant="outline" onClick={exportCsv} disabled={!data}><Download aria-hidden="true" />Export to CSV</Button></header>
      <form onSubmit={(event) => event.preventDefault()} className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <div className="flex flex-col gap-2"><Label htmlFor="analytics-year">Year</Label><NativeSelect id="analytics-year" value={year} onChange={(event) => setYear(Number(event.target.value))}>{years.map((option) => <option key={option} value={option}>{option}</option>)}</NativeSelect></div>
        <div className="flex flex-col gap-2"><Label htmlFor="analytics-period">Period</Label><NativeSelect id="analytics-period" value={period} onChange={(event) => setPeriod(event.target.value as AnalyticsPeriod)}>{ANALYTICS_PERIODS.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}</NativeSelect></div>
        <div className="flex flex-col gap-2"><Label htmlFor="analytics-metric">Metric</Label><NativeSelect id="analytics-metric" value={metric} onChange={(event) => setMetric(event.target.value)}>{METRICS.map((option) => <option key={option.key} value={option.key}>{option.label}</option>)}</NativeSelect></div>
        <div className="flex flex-col gap-2"><Label htmlFor="analytics-status">Status</Label><NativeSelect id="analytics-status" value={status} onChange={(event) => setStatus(event.target.value as AttendanceStatus)}><option value="approved">Approved</option><option value="all">All Submitted</option></NativeSelect></div>
        <label htmlFor="analytics-compare" className="flex min-h-11 items-center gap-3 text-sm sm:col-span-2 xl:col-span-4"><input id="analytics-compare" type="checkbox" className="size-5 accent-[var(--primary)]" checked={compare} onChange={(event) => setCompare(event.target.checked)} /><span>Compare with previous year</span></label>
      </form>
      {isPending ? <Loading /> : error ? <ErrorRetry message={error.message} onRetry={() => refetch()} /> : (
        <>
          <section aria-label="Analytics summary" className="grid gap-4 md:grid-cols-3">
            <Card size="sm"><CardHeader><CardDescription>Total {periodLabel(period)} Sum</CardDescription><CardTitle className="font-mono text-2xl tabular-nums">{formatValue(yearlyTotal)}</CardTitle></CardHeader><CardContent className="text-muted-foreground">{metricLabel} · {year}{comparison && <p>{changeText(yearlyTotal, previousTotal)} · vs {comparison.year}</p>}</CardContent></Card>
            <Card size="sm"><CardHeader><CardDescription>Peak Month</CardDescription><CardTitle className="text-2xl">{peakMonth[0] || "—"}</CardTitle></CardHeader><CardContent className="text-muted-foreground">{peakMonth[0] ? `${formatValue(peakMonth[1])} ${metricLabel.toLowerCase()}` : "No data available"}{comparison && <p>{changeText(peakMonth[1], previousPeakMonth?.[1])}</p>}</CardContent></Card>
            <Card size="sm"><CardHeader><CardDescription>Active Churches</CardDescription><CardTitle className="text-2xl">{activeChurches} out of {stats?.churches_total ?? "—"} churches</CardTitle></CardHeader><CardContent className="text-muted-foreground">Reported a value greater than 0{comparison && <p>{changeText(activeChurches, previousActiveChurches)}</p>}</CardContent></Card>
          </section>
          <div className="grid gap-6 xl:grid-cols-2">
            <Card><CardHeader><CardTitle>Weekly trend</CardTitle><CardDescription>{metricLabel} across all churches · {periodLabel(period)} {year} · {status === "approved" ? "Approved" : "All Submitted"}</CardDescription></CardHeader><CardContent>{!hasTrendValues ? <p role="status" className="flex h-72 items-center justify-center text-sm text-muted-foreground">No data available for this selection</p> : <div aria-label={`${metricLabel} weekly trend chart`} role="img" className="h-72 w-full"><ResponsiveContainer width="100%" height="100%"><LineChart data={trendData} margin={{ top: 8, right: 8, left: 0, bottom: 8 }}><CartesianGrid strokeDasharray="3 3" stroke="var(--border)" /><XAxis dataKey="label" interval="preserveStartEnd" tick={{ fontSize: 11 }} /><YAxis allowDecimals={false} label={{ value: metricLabel, angle: -90, position: "insideLeft" }} /><Tooltip content={renderTooltip} /><Line type="monotone" dataKey="value" name={`${year} ${metricLabel}`} stroke="var(--primary)" strokeWidth={2} dot={{ r: 3, fill: "var(--primary)" }} />{comparison && <Line type="monotone" dataKey="previousValue" name={`${comparison.year} ${metricLabel}`} stroke="var(--muted-foreground)" strokeDasharray="6 4" strokeWidth={2} dot={false} />}</LineChart></ResponsiveContainer></div>}</CardContent></Card>
            <Card><CardHeader><CardTitle>Church breakdown</CardTitle><CardDescription>Total {metricLabel} reported by church · {periodLabel(period)} {year}</CardDescription></CardHeader><CardContent>{!hasChurchValues ? <p role="status" className="flex h-72 items-center justify-center text-sm text-muted-foreground">No data available for this selection</p> : <div aria-label={`${metricLabel} church breakdown chart`} role="img" className="h-72 w-full"><ResponsiveContainer width="100%" height="100%"><BarChart data={churchData} layout="vertical" margin={{ top: 8, right: 8, left: 8, bottom: 8 }}><CartesianGrid strokeDasharray="3 3" stroke="var(--border)" /><XAxis type="number" allowDecimals={false} label={{ value: metricLabel, position: "insideBottom", offset: -4 }} /><YAxis dataKey="church_name" type="category" width={100} tick={{ fontSize: 11 }} /><Tooltip content={renderTooltip} /><Bar dataKey="value" name={`${year} ${metricLabel}`} fill="var(--primary)" />{comparison && <Bar dataKey="previousValue" name={`${comparison.year} ${metricLabel}`} fill="var(--muted-foreground)" />}</BarChart></ResponsiveContainer></div>}</CardContent></Card>
          </div>
          <section aria-labelledby="analytics-data-heading" className="flex flex-col gap-4"><h2 id="analytics-data-heading" className="text-base font-medium">Data table</h2><Table aria-label="Weekly trend data"><TableCaption>Weekly {metricLabel} totals for {periodLabel(period)} {year}.</TableCaption><TableHeader><TableRow><TableHead>Month</TableHead><TableHead>Week</TableHead><TableHead className="text-right">{year}</TableHead>{comparison && <TableHead className="text-right">{comparison.year}</TableHead>}</TableRow></TableHeader><TableBody>{trendWithData.map((point) => <TableRow key={`${point.month}-${point.week}`}><TableCell>{point.month}</TableCell><TableCell>{point.week}</TableCell><TableCell className="text-right tabular-nums">{formatValue(point.value)}</TableCell>{comparison && <TableCell className="text-right tabular-nums">{formatValue(point.previousValue)}</TableCell>}</TableRow>)}</TableBody></Table><Table aria-label="Church breakdown data"><TableCaption>Total {metricLabel} by church for {periodLabel(period)} {year}.</TableCaption><TableHeader><TableRow><TableHead>Church</TableHead><TableHead className="text-right">{year}</TableHead>{comparison && <TableHead className="text-right">{comparison.year}</TableHead>}</TableRow></TableHeader><TableBody>{churchData.map((church) => <TableRow key={church.church_name}><TableCell>{church.church_name}</TableCell><TableCell className="text-right tabular-nums">{formatValue(church.value)}</TableCell>{comparison && <TableCell className="text-right tabular-nums">{formatValue(church.previousValue)}</TableCell>}</TableRow>)}</TableBody></Table></section>
        </>
      )}
    </div>
  );
}
