"use client";

import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { CircleCheck, Clock, FilePen, FileText, FileWarning, Send } from "lucide-react";
import { cn } from "cn";
import { UserStatusBadge } from "@/components/user-status-badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { NativeSelect } from "@/components/ui/native-select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { ApiError } from "@/lib/api";
import { defaultPeriod, FIRST_YEAR, MONTHS } from "@/lib/reports";
import { getTracker, REVIEW_STATUSES, SUBMISSION_STATUSES, trackerCounts, trackerLabel } from "@/lib/tracker";
import { EmptyState } from "@/components/empty-state";
import { StatCard } from "@/components/stat-card";
import { KpiSkeleton, TableSkeleton } from "@/components/loading-skeletons";
import { ErrorRetry } from "@/components/error-retry";

// Light tint with dark text (and the reverse in the dark theme) to stay above 4.5:1 for axe.
// The label is always printed, so colour is never the only cue.
const TONE: Record<string, string> = {
  approved: "bg-primary/10 text-primary",
  pending: "bg-yellow-100 text-yellow-900 dark:bg-yellow-950 dark:text-yellow-200",
  needs_revision: "bg-red-100 text-red-900 dark:bg-red-950 dark:text-red-200",
  no_report: "bg-red-100 text-red-900 dark:bg-red-950 dark:text-red-200",
};

// The Tracker tab: who has filed a report for one Manila month (D-A3). It starts on the previous
// month, as Reports does, because workers file last month's report.
// shortcut: search and the status filter run in the browser over the whole month (about 24 accounts);
// move them to the server if the accounts grow into the hundreds.
export function Tracker() {
  const [period, setPeriod] = useState<{ month: string; year: number }>(() => defaultPeriod());
  const [search, setSearch] = useState("");
  const [submissionStatus, setSubmissionStatus] = useState("");
  const [reviewStatus, setReviewStatus] = useState("");

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
      (!submissionStatus || r.submission_status === submissionStatus) &&
      (!reviewStatus || r.review_status === reviewStatus) &&
      (!q || [r.name, r.email, r.church_name].some((v) => v.toLowerCase().includes(q))),
  );

  return (
    <div className="flex flex-col gap-6">
      <form onSubmit={(e) => e.preventDefault()} className="grid gap-4 sm:grid-cols-2 lg:grid-cols-5">
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
          <Label htmlFor="tracker-submission-status">Submission status</Label>
          <NativeSelect id="tracker-submission-status" value={submissionStatus} onChange={(e) => setSubmissionStatus(e.target.value)}>
            <option value="">All submission statuses</option>
            {SUBMISSION_STATUSES.map((s) => (
              <option key={s.code} value={s.code}>{s.label}</option>
            ))}
          </NativeSelect>
        </div>
        <div className="flex flex-col gap-2">
          <Label htmlFor="tracker-review-status">Review status</Label>
          <NativeSelect id="tracker-review-status" value={reviewStatus} onChange={(e) => setReviewStatus(e.target.value)}>
            <option value="">All review statuses</option>
            {REVIEW_STATUSES.map((s) => (
              <option key={s.code} value={s.code}>
                {s.label}
              </option>
            ))}
          </NativeSelect>
        </div>
      </form>

      {isPending ? (
        <div className="flex flex-col gap-6"><KpiSkeleton count={6} /><TableSkeleton columns={4} /></div>
      ) : error ? (
        <ErrorRetry message={error.message} onRetry={() => refetch()} />
      ) : rows.length === 0 ? (
        <EmptyState title="No church workers to track" description="Active church workers will appear here once they are assigned." />
      ) : (
        <>
          <div className="grid gap-6 xl:grid-cols-2">
            <section aria-label="Submission summary">
              <h2 className="mb-2 text-sm font-medium">Submission status</h2>
              <div className="grid gap-4 sm:grid-cols-3">
                <StatCard label="Expected" value={counts.expected} icon={FileText} tone="neutral" />
                <StatCard label="Submitted" value={counts.submitted} icon={Send} tone="sky" />
                <StatCard label="No Report" value={counts.noReport} icon={FileWarning} tone="amber" />
              </div>
            </section>
            <section aria-label="Review summary">
              <h2 className="mb-2 text-sm font-medium">Review status</h2>
              <div className="grid gap-4 sm:grid-cols-3">
                <StatCard label="Approved" value={counts.approved} icon={CircleCheck} tone="green" />
                <StatCard label="Pending" value={counts.pending} icon={Clock} tone="amber" />
                <StatCard label="Needs Revision" value={counts.needsRevision} icon={FilePen} tone="red" />
              </div>
            </section>
          </div>
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
            <EmptyState title="No workers match" description="Try clearing the search or status filters." />
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Worker</TableHead>
                  <TableHead>Church</TableHead>
                  <TableHead>Submission Status</TableHead>
                  <TableHead>Review Status</TableHead>
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
                          TONE[r.submission_status] ?? "bg-muted text-foreground",
                        )}
                      >
                        {trackerLabel(r.submission_status)}
                      </span>
                    </TableCell>
                    <TableCell>
                      <span
                        className={cn(
                          "inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium whitespace-nowrap",
                          r.review_status ? TONE[r.review_status] ?? "bg-muted text-foreground" : "text-muted-foreground",
                        )}
                      >
                        {r.review_status ? trackerLabel(r.review_status) : "—"}
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
