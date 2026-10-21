"use client";

import { keepPreviousData, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { toast } from "sonner";
import { Can } from "@/components/can";
import { ReportOwnerBadges } from "@/components/report-owner-badges";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { NativeSelect } from "@/components/ui/native-select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { ApiError } from "@/lib/api";
import { retryText } from "@/lib/auth-form";
import { defaultPeriod, FIRST_YEAR, listReports, MONTHS, toggleComplete, type Report } from "@/lib/reports";
import { useSession } from "@/lib/session";

// Which report a row control is for, so a screen reader user can tell the rows apart.
const rowName = (r: Report) => `${r.month} ${r.year}${r.worker ? `, ${r.worker}` : ""}`;

// The Reports tab of the admin panel: one month at a time, chosen with the Month and Year dropdowns.
// They live in state, not the URL, and start on the previous month, the same period as the stats cards.
export function AdminReports() {
  const { can } = useSession();
  const allowed = can("reports:read");

  const [period, setPeriod] = useState<{ month: string; year: number }>(defaultPeriod);
  const [page, setPage] = useState(1);

  const filters = { month: period.month, year: period.year, page: page > 1 ? page : undefined };
  const { data, error, isPending } = useQuery({
    queryKey: ["reports", filters], // the same prefix as /reports, so one invalidation reaches both
    queryFn: () => listReports(filters),
    enabled: allowed,
    placeholderData: keepPreviousData,
    staleTime: 30_000,
  });

  // Rows whose approve or unapprove is in flight. The endpoint toggles, so a second click before the answer would undo the first.
  const queryClient = useQueryClient();
  const [busy, setBusy] = useState<Set<number>>(new Set());

  // The endpoint toggles: the button says which way the row goes, and the server has the last word.
  async function approve(r: Report) {
    const when = `${r.month} ${r.year}`;
    setBusy((s) => new Set(s).add(r.id));
    try {
      const meta = await toggleComplete(r.id);
      // The server decides (another admin may have approved it first, and this click undid that), so say what it did.
      toast.success(`${meta.completed ? "Approved" : "Unapproved"} ${when}`);
      queryClient.invalidateQueries({ queryKey: ["admin-stats"] }); // the cards above count these reports
      await queryClient.invalidateQueries({ queryKey: ["reports"] }); // the row only changes once this answers, so hold the button
    } catch (err) {
      if (err instanceof ApiError && err.status === 404) {
        toast.error(`${when} no longer exists.`);
        await queryClient.invalidateQueries({ queryKey: ["reports"] });
      } else {
        toast.error(err instanceof ApiError ? retryText(err) : "Could not update the report. Try again.");
      }
    } finally {
      setBusy((s) => {
        const n = new Set(s);
        n.delete(r.id);
        return n;
      });
    }
  }

  const thisYear = new Date().getFullYear();
  const years = Array.from({ length: thisYear - FIRST_YEAR + 1 }, (_, i) => FIRST_YEAR + i);

  if (!allowed || (error instanceof ApiError && error.status === 403)) {
    return <p className="text-sm text-muted-foreground">You are not allowed to view reports.</p>;
  }

  const meta = data?.meta;
  return (
    <div className="flex flex-col gap-6">
      <form onSubmit={(e) => e.preventDefault()} className="flex flex-wrap items-end gap-4">
        <div className="flex flex-col gap-2">
          <Label htmlFor="month">Month</Label>
          <NativeSelect
            id="month"
            value={period.month}
            onChange={(e) => {
              setPeriod({ ...period, month: e.target.value });
              setPage(1);
            }}
          >
            {MONTHS.map((m) => (
              <option key={m} value={m}>
                {m}
              </option>
            ))}
          </NativeSelect>
        </div>
        <div className="flex flex-col gap-2">
          <Label htmlFor="year">Year</Label>
          <NativeSelect
            id="year"
            value={period.year}
            onChange={(e) => {
              setPeriod({ ...period, year: Number(e.target.value) });
              setPage(1);
            }}
          >
            {years.map((y) => (
              <option key={y} value={y}>
                {y}
              </option>
            ))}
          </NativeSelect>
        </div>
        {/* A placeholder: the export is Phase C (D-A6), so there is no endpoint to call yet. */}
        <Button type="button" variant="outline" className="ml-auto" disabled title="Excel export is not available yet">
          Download Bulk
        </Button>
      </form>

      {isPending ? (
        <p role="status" className="text-sm text-muted-foreground">Loading…</p>
      ) : error ? (
        <p role="alert" className="text-sm text-destructive">
          {error.message}
        </p>
      ) : data && data.data.length === 0 ? (
        <p role="status" className="text-sm text-muted-foreground">No reports match these filters.</p>
      ) : (
        data && (
          <>
            <p role="status" className="text-sm text-muted-foreground">
              {meta?.total} total reports
            </p>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Worker</TableHead>
                  <TableHead>Month</TableHead>
                  <TableHead>Year</TableHead>
                  <TableHead>Area of Assignment</TableHead>
                  <TableHead>Church</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Action</TableHead>
                  <TableHead>Download</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {data.data.map((r) => (
                  <TableRow key={r.id}>
                    <TableCell>
                      <div className="flex flex-wrap items-center gap-2">
                        <span>{r.worker || "—"}</span>
                        <ReportOwnerBadges report={r} />
                      </div>
                    </TableCell>
                    <TableCell>{r.month}</TableCell>
                    <TableCell>{r.year}</TableCell>
                    <TableCell>{r.area_assignment || "—"}</TableCell>
                    <TableCell>{r.church_name || "—"}</TableCell>
                    <TableCell>
                      <Badge variant={r.completed ? "default" : "secondary"}>{r.completed ? "Completed" : "Pending"}</Badge>
                    </TableCell>
                    <TableCell>
                      <Can permission="reports:approve" fallback="—">
                        <Button
                          size="sm"
                          variant={r.completed ? "outline" : "default"}
                          className={r.completed ? undefined : "bg-green-700 text-white hover:bg-green-800"}
                          disabled={busy.has(r.id)}
                          onClick={() => approve(r)}
                        >
                          {r.completed ? "Unapprove" : "Approve"} <span className="sr-only">{rowName(r)}</span>
                        </Button>
                      </Can>
                    </TableCell>
                    <TableCell>
                      {/* A placeholder: the export is Phase C (D-A6). A disabled button, not a link, because there is no URL. */}
                      <Button size="sm" variant="outline" disabled title="Excel export is not available yet">
                        Excel <span className="sr-only">{rowName(r)}</span>
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
            {meta && meta.total_pages! > 1 && (
              <div className="flex items-center justify-between gap-4">
                <p role="status" className="text-sm text-muted-foreground">
                  Page {meta.page} of {meta.total_pages}
                </p>
                <div className="flex gap-2">
                  <Button variant="outline" disabled={meta.page! <= 1} onClick={() => setPage(meta.page! - 1)}>
                    Previous
                  </Button>
                  <Button variant="outline" disabled={meta.page! >= meta.total_pages!} onClick={() => setPage(meta.page! + 1)}>
                    Next
                  </Button>
                </div>
              </div>
            )}
          </>
        )
      )}
    </div>
  );
}
