"use client";

import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import Link from "next/link";
import { Bell, Check, Download, Eye, Pencil, Trash2, Undo2 } from "lucide-react";
import { ReportsHeader, ReportsStats, ReportsTableCard } from "@/components/reports-dashboard";
import { AuthField } from "@/components/auth-field";
import { Can } from "@/components/can";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { NativeSelect } from "@/components/ui/native-select";
import { ApiError } from "@/lib/api";
import {
  defaultPeriod,
  deleteReport,
  exportReport,
  exportReports,
  FIRST_YEAR,
  listReports,
  MONTHS,
  type Report,
} from "@/lib/reports";
import { useSession } from "@/lib/session";
import { ExcelPreviewDialog } from "./excel-preview-dialog";
import { ReportPreviewDialog } from "./report-preview-dialog";
import { Loading } from "@/components/loading";
import { ErrorRetry } from "@/components/error-retry";
import { useReportActions } from "@/components/use-report-actions";
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { errorText } from "@/lib/auth-form";
import { refreshAdminViews } from "@/lib/query";

// Which report a row control is for, so a screen reader user can tell the rows apart.
const rowName = (r: Report) => `${r.month} ${r.year}${r.worker ? `, ${r.worker}` : ""}`;

// The Reports tab of the admin panel: one month at a time, chosen with the Month and Year dropdowns.
// They live in state, not the URL, and start on the previous month, the same period as the stats cards.
export function AdminReports() {
  const { can, user } = useSession();
  const allowed = can("reports:read");

  const [period, setPeriod] = useState<{ month: string; year: number }>(defaultPeriod);
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState({ worker: "", church: "", area: "" });
  const [draft, setDraft] = useState(search);

  useEffect(() => {
    if (draft === search) return;
    const timer = setTimeout(() => {
      setSearch(draft);
      setPage(1);
    }, 300);
    return () => clearTimeout(timer);
  }, [draft, search]);

  const filters = { ...search, month: period.month, year: period.year, page: page > 1 ? page : undefined };
  const { data, error, isPending, refetch } = useQuery({
    queryKey: ["reports", filters], // the same prefix as /reports, so one invalidation reaches both
    queryFn: () => listReports(filters),
    enabled: allowed,
    placeholderData: keepPreviousData,
    staleTime: 30_000,
  });

  const { approve, busy, download, downloading, preview, setPreview, reportPreview, setReportPreview } = useReportActions();
  const queryClient = useQueryClient();
  const [target, setTarget] = useState<Report | null>(null);
  const del = useMutation({
    mutationFn: (id: number) => deleteReport(id),
    onSuccess: async () => {
      setTarget(null);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["reports"] }),
        refreshAdminViews(queryClient),
      ]);
    },
  });

  const thisYear = new Date().getFullYear();
  const years = Array.from({ length: thisYear - FIRST_YEAR + 1 }, (_, i) => FIRST_YEAR + i);

  if (!allowed || (error instanceof ApiError && error.status === 403)) {
    return <p className="text-sm text-muted-foreground">You are not allowed to view reports.</p>;
  }

  const meta = data?.meta;
  return (
    <div className="flex flex-col gap-6">
      <ReportsHeader title="All Reports">
        <Can permission="reports:export">
          <Button
            type="button"
            variant="outline"
            className="min-h-11"
            disabled={downloading.includes("bulk") || !data?.data.length}
            aria-busy={downloading.includes("bulk")}
            onClick={() =>
              setPreview({ key: "bulk", target: { ...period }, run: () => exportReports(period.month, period.year) })
            }
          >
            <Download aria-hidden="true" />
            {downloading.includes("bulk") ? "Downloading…" : "Download Bulk"}
          </Button>
        </Can>
        <Button variant="outline" className="size-11" disabled aria-label="Notifications — email alerts coming soon" title="Email alerts coming soon">
          <Bell aria-hidden="true" />
        </Button>
      </ReportsHeader>
      <ReportsStats data={error ? undefined : data} />
      <ReportsTableCard rows={error || isPending ? [] : data?.data ?? []} scope={JSON.stringify(filters)} selectable={can("reports:approve") || can("reports:export") || can("reports:write")} filters={(
      <form onSubmit={(e) => e.preventDefault()} className="grid items-end gap-4 sm:grid-cols-2 xl:grid-cols-6">
        <div className="flex flex-col gap-2">
          <Label htmlFor="admin-month">Month</Label>
          <NativeSelect
            id="admin-month"
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
          <Label htmlFor="admin-year">Year</Label>
          <NativeSelect
            id="admin-year"
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
        {(["worker", "church", "area"] as const).map((name) => (
          <AuthField
            key={name}
            name={`admin-${name}`}
            label={name[0].toUpperCase() + name.slice(1)}
            placeholder={{ worker: "e.g. Juan Dela Cruz", church: "e.g. AMGC Church", area: "e.g. Metro Manila" }[name]}
            value={draft[name]}
            onChange={(e) => setDraft({ ...draft, [name]: e.target.value })}
          />
        ))}
        {Object.values(draft).some(Boolean) && (
          <Button
            type="button"
            variant="outline"
            className="min-h-11"
            onClick={() => {
              const empty = { worker: "", church: "", area: "" };
              setDraft(empty);
              setSearch(empty);
              setPage(1);
            }}
          >
            Clear
          </Button>
        )}

      </form>

      )} actions={(r) => (
        <>
          <Button size="icon" className="size-11" variant="ghost" aria-label={`Preview ${rowName(r)}`} title="Preview" onClick={() => setReportPreview(r.id)}><Eye aria-hidden="true" /></Button>
          {can("reports:write") && (r.created_by === user?.user_id || can("reports:write_any")) && <Button asChild size="icon" className="size-11" variant="ghost"><Link href={`/reports/${r.id}/edit`} aria-label={`Edit ${rowName(r)}`} title="Edit"><Pencil aria-hidden="true" /></Link></Button>}
          {can("reports:delete") && (r.created_by === user?.user_id || can("reports:delete_any")) && <Button size="icon" className="size-11 text-destructive" variant="ghost" aria-label={`Delete ${rowName(r)}`} title="Delete" onClick={() => { del.reset(); setTarget(r); }}><Trash2 aria-hidden="true" /></Button>}
          <Can permission="reports:approve"><Button size="icon" variant="ghost" className="size-11 text-[var(--approved)]" aria-label={`${r.completed ? "Unapprove" : "Approve"} ${rowName(r)}`} title={r.completed ? "Unapprove" : "Approve"} disabled={busy.includes(r.id)} aria-busy={busy.includes(r.id)} onClick={() => approve.mutate(r)}>{r.completed ? <Undo2 aria-hidden="true" /> : <Check aria-hidden="true" />}</Button></Can>
          <Can permission="reports:export"><Button size="icon" className="size-11" variant="ghost" aria-label={`Excel ${rowName(r)}`} title="Excel export" disabled={downloading.includes(String(r.id))} aria-busy={downloading.includes(String(r.id))} onClick={() => setPreview({ key: String(r.id), target: { reportId: r.id, label: r.church_name || `Report ${r.id}` }, run: () => exportReport(r.id) })}><Download aria-hidden="true" /></Button></Can>
        </>
      )}>

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
            {meta && meta.total_pages! > 0 && (
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
      </ReportsTableCard>
      {preview && (
        <ExcelPreviewDialog
          target={preview.target}
          onDownload={() => download.mutateAsync(preview).then(() => true, () => false)}
          onClose={() => setPreview(null)}
        />
      )}
      {reportPreview !== null && (
        <ReportPreviewDialog
          reportId={reportPreview}
          onClose={() => setReportPreview(null)}
        />
      )}
      <AlertDialog open={target !== null} onOpenChange={(open) => !open && !del.isPending && setTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete {target ? rowName(target) : "report"}?</AlertDialogTitle>
            <AlertDialogDescription>You can file this month again afterwards.</AlertDialogDescription>
          </AlertDialogHeader>
          {del.error && <p role="alert" className="text-sm text-destructive">{errorText(del.error, "Could not delete the report. Try again.")}</p>}
          <AlertDialogFooter>
            <AlertDialogCancel disabled={del.isPending}>Cancel</AlertDialogCancel>
            <Button variant="destructive" disabled={del.isPending} onClick={() => target && del.mutate(target.id)}>
              {del.isPending ? "Deleting…" : "Delete"}
            </Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
