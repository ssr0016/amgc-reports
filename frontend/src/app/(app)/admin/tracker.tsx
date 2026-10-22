"use client";

import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { cn } from "cn";
import { UserStatusBadge } from "@/components/user-status-badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { NativeSelect } from "@/components/ui/native-select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { ApiError } from "@/lib/api";
import { defaultPeriod, FIRST_YEAR, MONTHS } from "@/lib/reports";
import { getTracker, TRACKER_STATUSES, trackerCounts, trackerLabel } from "@/lib/tracker";

// Light tint with dark text (and the reverse in the dark theme) to stay above 4.5:1 for axe.
// The label is always printed, so colour is never the only cue.
const TONE: Record<string, string> = {
  approved: "bg-green-100 text-green-900 dark:bg-green-950 dark:text-green-200",
  for_review: "bg-yellow-100 text-yellow-900 dark:bg-yellow-950 dark:text-yellow-200",
  not_submitted: "bg-red-100 text-red-900 dark:bg-red-950 dark:text-red-200",
};

// The Tracker tab: who has filed a report for one Manila month (D-A3). It starts on the previous
// month, as Reports does, because workers file last month's report.
// shortcut: search and the status filter run in the browser over the whole month (about 24 accounts);
// move them to the server if the accounts grow into the hundreds.
export function Tracker() {
  const [period, setPeriod] = useState<{ month: string; year: number }>(() => defaultPeriod());
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("");

  const { data, error, isPending, refetch } = useQuery({
    queryKey: ["tracker", period.month, period.year],
    queryFn: () => getTracker(period.month, period.year),
    placeholderData: keepPreviousData,
    staleTime: 30_000,
  });

  const thisYear = new Date().getFullYear();
  const years = Array.from({ length: thisYear - FIRST_YEAR + 1 }, (_, i) => FIRST_YEAR + i);

  if (error instanceof ApiError && error.status === 403) {
    return <p className="text-sm text-muted-foreground">You are not allowed to view the tracker.</p>;
  }

  const rows = data?.rows ?? [];
  const counts = trackerCounts(rows);
  const q = search.trim().toLowerCase();
  const shown = rows.filter(
    (r) =>
      (!status || r.status === status) &&
      (!q || [r.name, r.email, r.church_name].some((v) => v.toLowerCase().includes(q))),
  );

  return (
    <div className="flex flex-col gap-6">
      <form onSubmit={(e) => e.preventDefault()} className="flex flex-wrap items-end gap-4">
        <div className="flex flex-col gap-2">
          <Label htmlFor="tracker-month">Month</Label>
          <NativeSelect id="tracker-month" value={period.month} onChange={(e) => setPeriod((p) => ({ ...p, month: e.target.value }))}>
            {MONTHS.map((m) => (
              <option key={m} value={m}>
                {m}
              </option>
            ))}
          </NativeSelect>
        </div>
        <div className="flex flex-col gap-2">
          <Label htmlFor="tracker-year">Year</Label>
          <NativeSelect id="tracker-year" value={period.year} onChange={(e) => setPeriod((p) => ({ ...p, year: Number(e.target.value) }))}>
            {years.map((y) => (
              <option key={y} value={y}>
                {y}
              </option>
            ))}
          </NativeSelect>
        </div>
        <div className="flex flex-col gap-2">
          <Label htmlFor="tracker-search">Search</Label>
          <Input
            id="tracker-search"
            type="search"
            placeholder="Worker, email or church"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
        </div>
        <div className="flex flex-col gap-2">
          <Label htmlFor="tracker-status">Status</Label>
          <NativeSelect id="tracker-status" value={status} onChange={(e) => setStatus(e.target.value)}>
            <option value="">All statuses</option>
            {TRACKER_STATUSES.map((s) => (
              <option key={s.code} value={s.code}>
                {s.label}
              </option>
            ))}
          </NativeSelect>
        </div>
      </form>

      {isPending ? (
        <p role="status" className="text-sm text-muted-foreground">Loading…</p>
      ) : error ? (
        <div className="flex flex-wrap items-center gap-4">
          <p role="alert" className="text-sm text-destructive">
            {error.message}
          </p>
          <Button variant="outline" size="sm" onClick={() => refetch()}>
            Retry
          </Button>
        </div>
      ) : rows.length === 0 ? (
        <p role="status" className="text-sm text-muted-foreground">No church workers to track.</p>
      ) : (
        <>
          <dl className="grid grid-cols-2 gap-4 sm:grid-cols-5">
            {[
              ["Expected", counts.expected],
              ["Submitted", counts.submitted],
              ["Approved", counts.approved],
              ["For Review", counts.forReview],
              ["Not Submitted", counts.notSubmitted],
            ].map(([label, value]) => (
              <div key={label} className="rounded-lg border p-3">
                <dt className="text-xs text-muted-foreground">{label}</dt>
                <dd className="text-2xl font-semibold">{value}</dd>
              </div>
            ))}
          </dl>
          <div className="flex flex-col gap-2">
            <p id="tracker-progress-label" className="text-sm">
              {counts.submitted} of {counts.expected} submitted
            </p>
            <progress
              aria-labelledby="tracker-progress-label"
              value={counts.submitted}
              max={counts.expected}
              className="h-2 w-full overflow-hidden rounded-full [&::-moz-progress-bar]:bg-primary [&::-webkit-progress-bar]:bg-muted [&::-webkit-progress-value]:bg-primary"
            />
          </div>

          <p role="status" className="text-sm text-muted-foreground">
            {shown.length} of {rows.length} workers
          </p>
          {shown.length === 0 ? (
            <p className="text-sm text-muted-foreground">No workers match.</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Worker</TableHead>
                  <TableHead>Church</TableHead>
                  <TableHead>Status</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {shown.map((r) => (
                  <TableRow key={r.user_id}>
                    {/* Wrapping cells keep the table inside 320 px, so it needs no scroll container to reach by keyboard. */}
                    <TableCell className="whitespace-normal">
                      <div className="flex flex-col">
                        <span className="flex flex-wrap items-center gap-2">
                          {r.name}
                          {r.user_status !== "active" && <UserStatusBadge status={r.user_status} />}
                        </span>
                        <span className="break-all text-xs text-muted-foreground">{r.email}</span>
                      </div>
                    </TableCell>
                    <TableCell className="whitespace-normal">{r.church_name || "—"}</TableCell>
                    <TableCell>
                      <span
                        className={cn(
                          "inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium whitespace-nowrap",
                          TONE[r.status] ?? "bg-muted text-foreground",
                        )}
                      >
                        {trackerLabel(r.status)}
                      </span>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </>
      )}
    </div>
  );
}
