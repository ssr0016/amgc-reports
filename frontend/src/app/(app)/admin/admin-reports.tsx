"use client";

import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { toast } from "sonner";
import { Can } from "@/components/can";
import { ReportOwnerBadges } from "@/components/report-owner-badges";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { NativeSelect } from "@/components/ui/native-select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { ApiError, saveFile } from "@/lib/api";
import { errorText } from "@/lib/auth-form";
import {
  defaultPeriod,
  exportReport,
  exportReports,
  FIRST_YEAR,
  listReports,
  MONTHS,
  toggleComplete,
  type Report,
} from "@/lib/reports";
import { useSession } from "@/lib/session";
import { ExcelPreviewDialog, type PreviewTarget } from "./excel-preview-dialog";
import { Loading } from "@/components/loading";
import { ErrorRetry } from "@/components/error-retry";
import { refreshAdminViews, usePendingVariables } from "@/lib/query";

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
  const { data, error, isPending, refetch } = useQuery({
    queryKey: ["reports", filters], // the same prefix as /reports, so one invalidation reaches both
    queryFn: () => listReports(filters),
    enabled: allowed,
    placeholderData: keepPreviousData,
    staleTime: 30_000,
  });

  // The endpoint toggles: the button says which way the row goes, and the server has the last word.
  // A row stays busy until the list refetches, because a second click before the answer would undo the first.
  const queryClient = useQueryClient();
  const approve = useMutation({
    mutationKey: ["toggle-complete"],
    mutationFn: (r: Report) => toggleComplete(r.id),
    onSuccess: (meta, r) => {
      // The server decides (another admin may have approved it first, and this click undid that), so say what it did.
      toast.success(`Report ${meta.completed ? "approved" : "unapproved"}: ${r.month} ${r.year}`);
      refreshAdminViews(queryClient); // the cards above and the Tracker count these reports
      return queryClient.invalidateQueries({ queryKey: ["reports"] });
    },
    onError: (err, r) => {
      if (err instanceof ApiError && err.status === 404) {
        toast.error(`${r.month} ${r.year} no longer exists.`);
        return queryClient.invalidateQueries({ queryKey: ["reports"] });
      }
      toast.error(errorText(err, "Could not update the report. Try again."));
    },
  });
  const busy = usePendingVariables<Report>(["toggle-complete"]).map((r) => r.id);

  // Excel downloads, keyed "bulk" or by a row's id, so each button waits for its own file.
  const download = useMutation({
    mutationKey: ["excel-download"],
    mutationFn: ({ run }: { key: string; run: () => ReturnType<typeof exportReport> }) => run(),
    onSuccess: (file) => {
      saveFile(file);
      toast.success(`Downloaded ${file.filename}`);
    },
    onError: (err) => toast.error(errorText(err, "Could not download the file. Try again.")),
  });
  const downloading = usePendingVariables<{ key: string }>(["excel-download"]).map((d) => d.key);
  // The open Excel preview (D-A15): its target, and the download its Download button runs.
  const [preview, setPreview] = useState<{ key: string; target: PreviewTarget; run: () => ReturnType<typeof exportReport> } | null>(null);

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
        <Can permission="reports:export">
          <Button
            type="button"
            variant="outline"
            className="ml-auto"
            disabled={downloading.includes("bulk") || !data?.data.length}
            aria-busy={downloading.includes("bulk")}
            onClick={() =>
              setPreview({ key: "bulk", target: { ...period }, run: () => exportReports(period.month, period.year) })
            }
          >
            {downloading.includes("bulk") ? "Downloading…" : "Download Bulk"}
          </Button>
        </Can>
      </form>

      {isPending ? (
        <Loading />
      ) : error ? (
        <ErrorRetry message={error.message} onRetry={() => refetch()} />
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
                          disabled={busy.includes(r.id)}
                          onClick={() => approve.mutate(r)}
                        >
                          {r.completed ? "Unapprove" : "Approve"} <span className="sr-only">{rowName(r)}</span>
                        </Button>
                      </Can>
                    </TableCell>
                    <TableCell>
                      <Can permission="reports:export" fallback="—">
                        <Button
                          size="sm"
                          variant="outline"
                          disabled={downloading.includes(String(r.id))}
                          aria-busy={downloading.includes(String(r.id))}
                          onClick={() =>
                            setPreview({
                              key: String(r.id),
                              target: { reportId: r.id, label: r.church_name || `Report ${r.id}` },
                              run: () => exportReport(r.id),
                            })
                          }
                        >
                          {downloading.includes(String(r.id)) ? "Downloading…" : "Excel"} <span className="sr-only">{rowName(r)}</span>
                        </Button>
                      </Can>
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
      {preview && (
        <ExcelPreviewDialog
          target={preview.target}
          onDownload={() => download.mutateAsync(preview).then(() => true, () => false)}
          onClose={() => setPreview(null)}
        />
      )}
    </div>
  );
}
