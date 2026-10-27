"use client";

import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { Filter } from "lucide-react";
import { Fragment, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { NativeSelect } from "@/components/ui/native-select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { ApiError } from "@/lib/api";
import {
  ACTIONS,
  actionGroup,
  actionLabel,
  detailRows,
  formatTimestamp,
  listAuditLogs,
  type AuditLog,
} from "@/lib/audit-logs";
import { currentPeriod, FIRST_YEAR, MONTHS } from "@/lib/reports";
import { EmptyState } from "@/components/empty-state";
import { TableSkeleton } from "@/components/loading-skeletons";
import { ErrorRetry } from "@/components/error-retry";
import { Pagination } from "@/components/pagination";

const TONE = {
  create: "success",
  approve: "success",
  change: "warning",
  warning: "warning",
  delete: "danger",
  security: "info",
} as const;

const target = (l: AuditLog) => l.target_name || (l.target_id ? `#${l.target_id}` : "—");

// The Logs tab: the audit trail, one Manila month at a time, read-only. It starts on the month in
// progress (not the previous one, as Reports does), so today's sign-ins and changes show.
export function AuditLogs() {
  const [period, setPeriod] = useState<{ month: string; year: number }>(() => currentPeriod());
  const [action, setAction] = useState("");
  const [page, setPage] = useState(1);
  const [open, setOpen] = useState<Set<number>>(new Set());

  const filters = { month: period.month, year: period.year, action: action || undefined, page };
  const { data, error, isPending, refetch } = useQuery({
    queryKey: ["audit-logs", filters],
    queryFn: () => listAuditLogs(filters),
    placeholderData: keepPreviousData,
  });

  const thisYear = new Date().getFullYear();
  const years = Array.from({ length: thisYear - FIRST_YEAR + 1 }, (_, i) => FIRST_YEAR + i);

  // Any filter change starts again at page 1 with every row closed.
  function filter(next: Partial<{ month: string; year: number; action: string }>) {
    if (next.action !== undefined) setAction(next.action);
    setPeriod((p) => ({ month: next.month ?? p.month, year: next.year ?? p.year }));
    setPage(1);
    setOpen(new Set());
  }

  function toggle(id: number) {
    setOpen((s) => {
      const n = new Set(s);
      if (!n.delete(id)) n.add(id);
      return n;
    });
  }

  if (error instanceof ApiError && error.status === 403) {
    return <p className="text-sm text-muted-foreground">You are not allowed to view audit logs.</p>;
  }

  const meta = data?.meta;
  return (
    <Card className="gap-0 rounded-lg py-0">
      <form onSubmit={(e) => e.preventDefault()} className="grid items-end gap-5 border-b p-5 sm:grid-cols-2 sm:p-6 lg:grid-cols-3">
        <p className="col-span-full flex items-center gap-2 text-sm font-medium text-muted-foreground">
          <Filter aria-hidden="true" className="size-4" />
          Filter events
        </p>
        <div className="flex flex-col gap-2">
          <Label htmlFor="logs-month">Month</Label>
          <NativeSelect id="logs-month" value={period.month} onChange={(e) => filter({ month: e.target.value })}>
            {MONTHS.map((m) => (
              <option key={m} value={m}>
                {m}
              </option>
            ))}
          </NativeSelect>
        </div>
        <div className="flex flex-col gap-2">
          <Label htmlFor="logs-year">Year</Label>
          <NativeSelect id="logs-year" value={period.year} onChange={(e) => filter({ year: Number(e.target.value) })}>
            {years.map((y) => (
              <option key={y} value={y}>
                {y}
              </option>
            ))}
          </NativeSelect>
        </div>
        <div className="flex flex-col gap-2">
          <Label htmlFor="logs-action">Action</Label>
          <NativeSelect id="logs-action" value={action} onChange={(e) => filter({ action: e.target.value })}>
            <option value="">All actions</option>
            {ACTIONS.map((a) => (
              <option key={a.code} value={a.code}>
                {a.label}
              </option>
            ))}
          </NativeSelect>
        </div>
      </form>

      {/* The container must scroll vertically for the header to stick. */}
      <div className="min-w-0 p-5 sm:p-6 [&_[data-slot=table-container]]:max-h-[70vh]">
        {isPending ? (
          <TableSkeleton columns={4} />
        ) : error ? (
          <ErrorRetry message={error.message} onRetry={() => refetch()} />
        ) : data && data.data.length === 0 ? (
          <EmptyState title="No audit logs found" description="No activity matches the selected filters." />
        ) : (
          data && (
            <>
              <p role="status" className="mb-3 text-right text-sm text-muted-foreground tabular-nums">
                {meta?.total} total logs
              </p>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Timestamp</TableHead>
                    <TableHead>Actor</TableHead>
                    <TableHead>Action</TableHead>
                    <TableHead>Target</TableHead>
                    <TableHead>Details</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {data.data.map((l) => {
                    const expanded = open.has(l.id);
                    const panelId = `log-details-${l.id}`;
                    const rows = detailRows(l.action, l.details as Record<string, unknown>);
                    return (
                      <Fragment key={l.id}>
                        <TableRow>
                          <TableCell className="font-mono whitespace-nowrap tabular-nums">{formatTimestamp(l.created_at)}</TableCell>
                          <TableCell>
                            {l.performed_by_name ? (
                              <div className="flex flex-wrap items-center gap-2">
                                <span>{l.performed_by_name}</span>
                                {l.performed_by_role && (
                                  <span className="text-xs text-muted-foreground">{l.performed_by_role}</span>
                                )}
                              </div>
                            ) : (
                              "—"
                            )}
                          </TableCell>
                          <TableCell>
                            <Badge variant={TONE[actionGroup(l.action) as keyof typeof TONE] ?? "neutral"}>{actionLabel(l.action)}</Badge>
                          </TableCell>
                          <TableCell>
                            {l.target_name || l.target_id ? (
                              <div className="flex flex-col">
                                <span className="text-xs text-muted-foreground">{l.target_type}</span>
                                <span className="break-all">{target(l)}</span>
                              </div>
                            ) : (
                              "—"
                            )}
                          </TableCell>
                          <TableCell>
                            {/* A fixed label: aria-expanded tells whether the row is open. */}
                            <Button
                              size="sm"
                              variant="outline"
                              aria-expanded={expanded}
                              aria-controls={panelId}
                              onClick={() => toggle(l.id)}
                            >
                              View details
                              <span className="sr-only">
                                {" "}
                                for {actionLabel(l.action)}, {target(l)}, {formatTimestamp(l.created_at)}
                              </span>
                            </Button>
                          </TableCell>
                        </TableRow>
                        {expanded && (
                          <TableRow id={panelId} className="bg-muted/40 hover:bg-muted/40">
                            <TableCell colSpan={5} className="whitespace-normal">
                              <dl className="grid grid-cols-[max-content_1fr] gap-x-6 gap-y-1 text-sm">
                                <dt className="text-muted-foreground">Time</dt>
                                <dd className="font-mono tabular-nums">{formatTimestamp(l.created_at, true)}</dd>
                                <dt className="text-muted-foreground">Actor</dt>
                                <dd>
                                  {l.performed_by_name
                                    ? `${l.performed_by_name}${l.performed_by_role ? ` (${l.performed_by_role})` : ""}`
                                    : "—"}
                                </dd>
                                <dt className="text-muted-foreground">Target</dt>
                                <dd className="break-all">
                                  {l.target_type}
                                  {l.target_name ? `, ${l.target_name}` : ""}
                                  {l.target_id ? `, ID ${l.target_id}` : ""}
                                </dd>
                                {rows.map(([label, value]) => (
                                  <Fragment key={label}>
                                    <dt className="text-muted-foreground">{label}</dt>
                                    <dd className="break-words">{value}</dd>
                                  </Fragment>
                                ))}
                              </dl>
                              {rows.length === 0 && <p className="mt-2 text-sm text-muted-foreground">No further details.</p>}
                            </TableCell>
                          </TableRow>
                        )}
                      </Fragment>
                    );
                  })}
                </TableBody>
              </Table>
              {meta && <Pagination page={meta.page!} totalPages={meta.total_pages!} onPageChange={setPage} />}
            </>
          )
        )}
      </div>
    </Card>
  );
}
