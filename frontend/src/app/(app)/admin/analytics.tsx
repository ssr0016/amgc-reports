"use client";

import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { Bar, BarChart, CartesianGrid, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import type { TooltipContentProps } from "recharts";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { NativeSelect } from "@/components/ui/native-select";
import { Table, TableBody, TableCaption, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { ErrorRetry } from "@/components/error-retry";
import { Loading } from "@/components/loading";
import { getAttendanceAnalytics, type AttendanceStatus } from "@/lib/analytics";
import { METRICS } from "@/lib/report-form";
import { FIRST_YEAR } from "@/lib/reports";
import { useSession } from "@/lib/session";
import { getStats } from "@/lib/stats";

const formatValue = (value: number) => value.toLocaleString(undefined, { maximumFractionDigits: 2 });

export function Analytics() {
  const { can } = useSession();
  const allowed = can("tracker:read");
  const [year, setYear] = useState(() => new Date().getFullYear());
  const [metric, setMetric] = useState("worship_service");
  const [status, setStatus] = useState<AttendanceStatus>("approved");
  const { data, error, isPending, refetch } = useQuery({
    queryKey: ["analytics-attendance", year, metric, status],
    queryFn: () => getAttendanceAnalytics(year, metric, status),
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
  const trendWithData = trend.filter((point) => point.value !== 0);
  const churches = data?.churches ?? [];
  const metricLabel = METRICS.find((item) => item.key === metric)?.label ?? metric;
  const trendData = trend.map((point) => ({ ...point, label: `${point.month.slice(0, 3)} W${point.week}` }));
  const hasTrendValues = trend.some((point) => point.value !== 0);
  const hasChurchValues = churches.some((church) => church.value !== 0);
  const yearlyTotal = trend.reduce((total, point) => total + point.value, 0);
  const monthlyTotals = trend.reduce<Record<string, number>>((totals, point) => {
    totals[point.month] = (totals[point.month] ?? 0) + point.value;
    return totals;
  }, {});
  const peakMonth = Object.entries(monthlyTotals).reduce<[string, number]>(
    (peak, current) => current[1] > peak[1] ? current : peak,
    ["", 0],
  );
  const activeChurches = churches.filter((church) => church.value > 0).length;
  const renderTooltip = ({ active, payload }: TooltipContentProps) => {
    const item = payload?.[0];
    if (!active || !item) return null;
    const point = item.payload as { month?: string; week?: number; church_name?: string } | undefined;
    if (!point) return null;
    const label = point.month ? `${point.month}, Week ${point.week}` : point.church_name;
    return <p className="rounded-md border bg-background px-3 py-2 text-sm shadow-sm">{label}: {formatValue(Number(item.value))}</p>;
  };

  return (
    <div className="flex flex-col gap-6">
      <form onSubmit={(event) => event.preventDefault()} className="grid gap-4 sm:grid-cols-3">
        <div className="flex flex-col gap-2">
          <Label htmlFor="analytics-year">Year</Label>
          <NativeSelect id="analytics-year" value={year} onChange={(event) => setYear(Number(event.target.value))}>
            {years.map((option) => <option key={option} value={option}>{option}</option>)}
          </NativeSelect>
        </div>
        <div className="flex flex-col gap-2">
          <Label htmlFor="analytics-metric">Metric</Label>
          <NativeSelect id="analytics-metric" value={metric} onChange={(event) => setMetric(event.target.value)}>
            {METRICS.map((option) => <option key={option.key} value={option.key}>{option.label}</option>)}
          </NativeSelect>
        </div>
        <div className="flex flex-col gap-2">
          <Label htmlFor="analytics-status">Status</Label>
          <NativeSelect id="analytics-status" value={status} onChange={(event) => setStatus(event.target.value as AttendanceStatus)}>
            <option value="approved">Approved</option>
            <option value="all">All Submitted</option>
          </NativeSelect>
        </div>
      </form>

      {isPending ? <Loading /> : error ? <ErrorRetry message={error.message} onRetry={() => refetch()} /> : (
        <>
          <section aria-label="Analytics summary" className="grid gap-4 md:grid-cols-3">
            <Card size="sm">
              <CardHeader>
                <CardDescription>Total Yearly Sum</CardDescription>
                <CardTitle className="font-mono text-2xl tabular-nums">{formatValue(yearlyTotal)}</CardTitle>
              </CardHeader>
              <CardContent className="text-muted-foreground">{metricLabel} · {year}</CardContent>
            </Card>
            <Card size="sm">
              <CardHeader>
                <CardDescription>Peak Month</CardDescription>
                <CardTitle className="text-2xl">{peakMonth[0] || "—"}</CardTitle>
              </CardHeader>
              <CardContent className="text-muted-foreground">
                {peakMonth[0] ? `${formatValue(peakMonth[1])} ${metricLabel.toLowerCase()}` : "No data available"}
              </CardContent>
            </Card>
            <Card size="sm">
              <CardHeader>
                <CardDescription>Active Churches</CardDescription>
                <CardTitle className="text-2xl">{activeChurches} out of {stats?.churches_total ?? "—"} churches</CardTitle>
              </CardHeader>
              <CardContent className="text-muted-foreground">Reported a value greater than 0</CardContent>
            </Card>
          </section>

          <div className="grid gap-6 xl:grid-cols-2">
            <Card>
              <CardHeader>
                <CardTitle>Weekly trend</CardTitle>
                <CardDescription>{metricLabel} across all churches · {year} · {status === "approved" ? "Approved" : "All Submitted"}</CardDescription>
              </CardHeader>
              <CardContent>
                {!hasTrendValues ? <p role="status" className="flex h-72 items-center justify-center text-sm text-muted-foreground">No data available for this selection</p> : <div aria-label={`${metricLabel} weekly trend chart`} role="img" className="h-72 w-full">
                  <ResponsiveContainer width="100%" height="100%">
                    <LineChart data={trendData} margin={{ top: 8, right: 8, left: 0, bottom: 8 }}>
                      <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" />
                      <XAxis dataKey="label" interval="preserveStartEnd" tick={{ fontSize: 11 }} />
                      <YAxis allowDecimals={false} label={{ value: metricLabel, angle: -90, position: "insideLeft" }} />
                      <Tooltip content={renderTooltip} />
                      <Line type="monotone" dataKey="value" name={metricLabel} stroke="var(--primary)" strokeWidth={2} dot={{ r: 3, fill: "var(--primary)" }} />
                    </LineChart>
                  </ResponsiveContainer>
                </div>}
              </CardContent>
            </Card>
            <Card>
              <CardHeader>
                <CardTitle>Church breakdown</CardTitle>
                <CardDescription>Total {metricLabel} reported by church in {year}</CardDescription>
              </CardHeader>
              <CardContent>
                {!hasChurchValues ? <p role="status" className="flex h-72 items-center justify-center text-sm text-muted-foreground">No data available for this selection</p> : <div aria-label={`${metricLabel} church breakdown chart`} role="img" className="h-72 w-full">
                  <ResponsiveContainer width="100%" height="100%">
                    <BarChart data={churches} layout="vertical" margin={{ top: 8, right: 8, left: 8, bottom: 8 }}>
                      <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" />
                      <XAxis type="number" allowDecimals={false} label={{ value: metricLabel, position: "insideBottom", offset: -4 }} />
                      <YAxis dataKey="church_name" type="category" width={100} tick={{ fontSize: 11 }} />
                      <Tooltip content={renderTooltip} />
                      <Bar dataKey="value" name={metricLabel} fill="var(--primary)" />
                    </BarChart>
                  </ResponsiveContainer>
                </div>}
              </CardContent>
            </Card>
          </div>

          <section aria-labelledby="analytics-data-heading" className="flex flex-col gap-4">
            <h2 id="analytics-data-heading" className="text-base font-medium">Data table</h2>
            <Table aria-label="Weekly trend data">
              <TableCaption>Weekly {metricLabel} totals across all churches for {year}.</TableCaption>
              <TableHeader><TableRow><TableHead>Month</TableHead><TableHead>Week</TableHead><TableHead className="text-right">Total</TableHead></TableRow></TableHeader>
              <TableBody>{trendWithData.map((point) => <TableRow key={`${point.month}-${point.week}`}><TableCell>{point.month}</TableCell><TableCell>{point.week}</TableCell><TableCell className="text-right tabular-nums">{formatValue(point.value)}</TableCell></TableRow>)}</TableBody>
            </Table>
            <Table aria-label="Church breakdown data">
              <TableCaption>Total {metricLabel} by church for {year}.</TableCaption>
              <TableHeader><TableRow><TableHead>Church</TableHead><TableHead className="text-right">Total</TableHead></TableRow></TableHeader>
              <TableBody>{churches.map((church) => <TableRow key={church.church_name}><TableCell>{church.church_name}</TableCell><TableCell className="text-right tabular-nums">{formatValue(church.value)}</TableCell></TableRow>)}</TableBody>
            </Table>
          </section>
        </>
      )}
    </div>
  );
}
