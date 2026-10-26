"use client";

import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { Bar, BarChart, CartesianGrid, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
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

  if (!allowed) return <p className="text-sm text-muted-foreground">You are not allowed to view analytics.</p>;

  const currentYear = new Date().getFullYear();
  const years = Array.from({ length: Math.max(1, currentYear - FIRST_YEAR + 1) }, (_, i) => FIRST_YEAR + i);
  const trend = data?.trend ?? [];
  const trendWithData = trend.filter((point) => point.value !== 0);
  const churches = data?.churches ?? [];
  const metricLabel = METRICS.find((item) => item.key === metric)?.label ?? metric;
  const trendData = trend.map((point) => ({ ...point, label: `${point.month.slice(0, 3)} W${point.week}` }));
  const hasValues = trend.some((point) => point.value !== 0) || churches.some((church) => church.value !== 0);

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

      {isPending ? <Loading /> : error ? <ErrorRetry message={error.message} onRetry={() => refetch()} /> : !hasValues ? (
        <p role="status" className="text-sm text-muted-foreground">No attendance data for {metricLabel} in {year}.</p>
      ) : (
        <>
          <div className="grid gap-6 xl:grid-cols-2">
            <Card>
              <CardHeader>
                <CardTitle>Weekly trend</CardTitle>
                <CardDescription>{metricLabel} across all churches · {year} · {status === "approved" ? "Approved" : "All Submitted"}</CardDescription>
              </CardHeader>
              <CardContent>
                <div aria-label={`${metricLabel} weekly trend chart`} role="img" className="h-72 w-full">
                  <ResponsiveContainer width="100%" height="100%">
                    <LineChart data={trendData} margin={{ top: 8, right: 8, left: 0, bottom: 8 }}>
                      <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" />
                      <XAxis dataKey="label" interval="preserveStartEnd" tick={{ fontSize: 11 }} />
                      <YAxis allowDecimals={false} />
                      <Tooltip formatter={(value) => formatValue(Number(value))} />
                      <Line type="monotone" dataKey="value" name={metricLabel} stroke="var(--primary)" strokeWidth={2} dot={false} />
                    </LineChart>
                  </ResponsiveContainer>
                </div>
              </CardContent>
            </Card>
            <Card>
              <CardHeader>
                <CardTitle>Church breakdown</CardTitle>
                <CardDescription>Total {metricLabel} reported by church in {year}</CardDescription>
              </CardHeader>
              <CardContent>
                <div aria-label={`${metricLabel} church breakdown chart`} role="img" className="h-72 w-full">
                  <ResponsiveContainer width="100%" height="100%">
                    <BarChart data={churches} layout="vertical" margin={{ top: 8, right: 8, left: 8, bottom: 8 }}>
                      <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" />
                      <XAxis type="number" allowDecimals={false} />
                      <YAxis dataKey="church_name" type="category" width={100} tick={{ fontSize: 11 }} />
                      <Tooltip formatter={(value) => formatValue(Number(value))} />
                      <Bar dataKey="value" name={metricLabel} fill="var(--primary)" />
                    </BarChart>
                  </ResponsiveContainer>
                </div>
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
