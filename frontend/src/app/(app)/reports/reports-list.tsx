"use client";

import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, useEffect, useRef, useState } from "react";
import { toast } from "sonner";
import { AuthField } from "@/components/auth-field";
import { Can } from "@/components/can";
import { Check, Download, Eye, Loader2, Pencil, Trash2, Undo2 } from "lucide-react";
import { reportsTheme, ReportsHeader, ReportsTableCard } from "@/components/reports-dashboard";
import { useReportActions } from "@/components/use-report-actions";
import { ExcelPreviewDialog } from "../admin/excel-preview-dialog";
import { ReportPreviewDialog } from "../admin/report-preview-dialog";
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import { NativeSelect } from "@/components/ui/native-select";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { ApiError } from "@/lib/api";
import { errorText } from "@/lib/auth-form";
import {
  defaultPeriod,
  exportReport,
  deleteReport,
  getCurrentReport,
  FIRST_YEAR,
  listReports,
  MONTHS,
  reportAction,
  type Report,
  type ReportFilters,
} from "@/lib/reports";
import { useSession } from "@/lib/session";
import { refreshAdminViews } from "@/lib/query";
import { clearReportDraftStorage } from "@/lib/report-draft-storage";
import { EmptyState } from "@/components/empty-state";
import { TableSkeleton } from "@/components/loading-skeletons";

const textFields = [
  ["worker", "e.g. Juan Dela Cruz"],
  ["area", "e.g. Metro Manila"],
  ["church", "e.g. AMGC Church"],
] as const;
type Text = Record<(typeof textFields)[number][0], string>;

// A month missing from the URL means the default; present but empty ("month=")
// means any month. The year is always one of FIRST_YEAR..this year.
function filtersFrom(params: URLSearchParams): ReportFilters {
  const def = defaultPeriod();
  const year = Number(params.get("year"));
  const page = Number(params.get("page"));
  return {
    month: (params.has("month") ? params.get("month") : def.month) || undefined,
    year: Number.isInteger(year) && year >= FIRST_YEAR && year <= new Date().getFullYear() ? year : def.year,
    worker: params.get("worker") || undefined,
    area: params.get("area") || undefined,
    church: params.get("church") || undefined,
    page: page > 1 ? page : undefined,
  };
}

// Which report a row action is for, so a screen reader user can tell the rows apart.
const rowName = (r: Report) => `${r.month} ${r.year}${r.worker ? `, ${r.worker}` : ""}`;

function ReportsList() {
  const router = useRouter();
  const params = useSearchParams();
  const { can, user } = useSession();
  const filters = filtersFrom(params);
  const allowed = can("reports:read");
  const { approve, busy, download, downloading, preview, setPreview, reportPreview, setReportPreview } = useReportActions();
  const [unapproveTarget, setUnapproveTarget] = useState<Report | null>(null);
  const [adminNotes, setAdminNotes] = useState("");
  const [metadataPreview, setMetadataPreview] = useState<Report | null>(null);
  const thisYear = new Date().getFullYear();
  const years = Array.from({ length: thisYear - FIRST_YEAR + 1 }, (_, i) => FIRST_YEAR + i);

  const { data, error, isPending } = useQuery({
    queryKey: ["reports", user?.user_id, filters],
    queryFn: () => listReports(filters),
    enabled: allowed,
    placeholderData: keepPreviousData,
    // Fresh for 30 s, so switching tabs back and forth does not refetch every time; the read limit is 60 a minute.
    staleTime: 30_000,
    refetchOnWindowFocus: true,
  });
  const currentReport = useQuery({
    queryKey: ["current-report", user?.user_id],
    queryFn: getCurrentReport,
    enabled: user?.role === "user" && can("reports:write"),
    staleTime: 30_000,
  });

  // The report the delete dialog is asking about, if any.
  const queryClient = useQueryClient();
  const [target, setTarget] = useState<{ report: Report; period: string } | null>(null);
  // The dialog is controlled and has no trigger element, so Radix cannot return focus
  // by itself: remember the Delete button that opened it (WCAG 2.4.3).
  const opener = useRef<HTMLElement | null>(null);

  const del = useMutation({
    mutationFn: (report: Report) =>
      deleteReport(report.id).catch((err) => {
        // 404: it was already deleted elsewhere, so the list below is just stale.
        if (!(err instanceof ApiError && err.status === 404)) throw err;
      }),
    onSuccess: async (_data, report) => {
      clearReportDraftStorage(report, user?.user_id);
      setTarget(null);
      toast.success("Report deleted successfully.");
      // Deleting the last row of a later page would leave an empty page behind.
      const lastOnPage = (filters.page ?? 1) > 1 && data?.data.length === 1;
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["current-report", user?.user_id] }),
        queryClient.invalidateQueries({ queryKey: ["reports"] }),
        refreshAdminViews(queryClient),
      ]);
      router.replace("/reports");
      if (lastOnPage) goToPage((filters.page ?? 1) - 1);
    },
  });
  const deleting = del.isPending;
  const deleteError = del.error ? errorText(del.error, "Could not delete the report. Try again.") : "";

  // Another user's report is readable (read_any) but only its owner, or someone with the _any permission, may change it.
  const mine = (r: Report) => r.created_by === user?.user_id;
  const canEdit = (r: Report) => can("reports:write") && (mine(r) || can("reports:write_any"));
  const canDelete = (r: Report) => can("reports:delete") && (mine(r) || can("reports:delete_any"));

  // Filters apply as they change. The text boxes wait 300 ms after the last key,
  // so typing does not spend the per-user read rate limit one request per letter.
  const urlText: Text = { worker: filters.worker ?? "", area: filters.area ?? "", church: filters.church ?? "" };
  const [text, setText] = useState<Text>(urlText);
  const urlKey = JSON.stringify(urlText);
  const sentKey = useRef(urlKey);

  function go(next: { month?: string; year: number } & Text, mode: "push" | "replace") {
    const q = new URLSearchParams({ month: next.month ?? "", year: String(next.year) });
    for (const [name] of textFields) if (next[name].trim()) q.set(name, next[name].trim());
    router[mode](`/reports?${q}`);
  }

  useEffect(() => {
    const key = JSON.stringify(text);
    if (key === sentKey.current) return;
    const timer = setTimeout(() => {
      sentKey.current = key;
      go({ month: filters.month, year: filters.year!, ...text }, "replace");
    }, 300);
    return () => clearTimeout(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- go() only reads the values listed here
  }, [text, filters.month, filters.year]);

  // The URL changed from outside (Back, a shared link): show its text in the boxes.
  useEffect(() => {
    if (urlKey === sentKey.current) return;
    sentKey.current = urlKey;
    setText(JSON.parse(urlKey));
  }, [urlKey]);

  function goToPage(page: number) {
    const next = new URLSearchParams(params);
    next.set("page", String(page));
    router.push(`/reports?${next}`);
  }

  if (!allowed || (error instanceof ApiError && error.status === 403)) {
    return <p className="text-sm text-muted-foreground">You are not allowed to view reports.</p>;
  }

  const meta = data?.meta;
  const hasText = textFields.some(([name]) => text[name] !== "");
  return (
    <>
      <ReportsHeader title="Reports" heading="h1">
        {user?.role === "user" && <Can permission="reports:write">{!currentReport.isPending && !currentReport.isError && <Button asChild className="min-h-11"><Link href={reportAction(currentReport.data).href}>{reportAction(currentReport.data).label}</Link></Button>}</Can>}
      </ReportsHeader>
      <ReportsTableCard rows={error || isPending ? [] : data?.data ?? []} selectable={false} filters={(
      <form onSubmit={(e) => e.preventDefault()} className="grid items-end gap-4 sm:grid-cols-2 lg:grid-cols-6">
        <div className="flex flex-col gap-2">
          <Label htmlFor="month">Month</Label>
          <NativeSelect
            id="month"
            value={filters.month ?? ""}
            onChange={(e) => go({ month: e.target.value, year: filters.year!, ...text }, "push")}
          >
            <option value="">All months</option>
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
            value={filters.year}
            onChange={(e) => go({ month: filters.month, year: Number(e.target.value), ...text }, "push")}
          >
            {years.map((y) => (
              <option key={y} value={y}>
                {y}
              </option>
            ))}
          </NativeSelect>
        </div>
        {textFields.map(([name, placeholder]) => (
          <AuthField
            key={name}
            name={name}
            label={name[0].toUpperCase() + name.slice(1)}
            placeholder={placeholder}
            autoComplete="off"
            value={text[name]}
            onChange={(e) => setText({ ...text, [name]: e.target.value })}
          />
        ))}
        {hasText && (
            <Button
              type="button"
              variant="outline"
              className="min-h-11"
              onClick={() => {
                const empty = { worker: "", area: "", church: "" };
                sentKey.current = JSON.stringify(empty);
                setText(empty);
                go({ month: filters.month, year: filters.year!, ...empty }, "push");
              }}
            >
              Clear
            </Button>
        )}
      </form>
      )} actions={(r) => (
        <>
          <Button variant="ghost" size="icon" className="size-11" aria-label={`Preview ${rowName(r)}`} title="Preview" onClick={(e) => { opener.current = e.currentTarget; if (mine(r) || can("reports:approve") || can("reports:write_any")) setReportPreview(r.id); else setMetadataPreview(r); }}><Eye aria-hidden="true" /></Button>
          {canEdit(r) && !can("reports:approve") && <Button asChild variant="ghost" size="icon" className="size-11"><Link href={`/reports/${r.id}/edit`} aria-label={`Edit ${rowName(r)}`} title="Edit"><Pencil aria-hidden="true" /></Link></Button>}
          <Can permission="reports:approve"><Button variant="ghost" size="icon" className="size-11 text-[var(--approved)]" aria-label={`${r.completed ? "Unapprove" : "Approve"} ${rowName(r)}`} title={r.completed ? "Unapprove" : "Approve"} disabled={busy.includes(r.id)} aria-busy={busy.includes(r.id)} onClick={() => { if (r.completed) { setAdminNotes(""); setUnapproveTarget(r); } else approve.mutate({ report: r }); }}>{busy.includes(r.id) ? <Loader2 aria-hidden="true" className="motion-safe:animate-spin" /> : r.completed ? <Undo2 aria-hidden="true" /> : <Check aria-hidden="true" />}</Button></Can>
          <Can permission="reports:export"><Button variant="ghost" size="icon" className="size-11" aria-label={`Excel ${rowName(r)}`} title="Excel export" disabled={downloading.includes(String(r.id))} aria-busy={downloading.includes(String(r.id))} onClick={() => setPreview({ key: String(r.id), target: { reportId: r.id, label: r.church_name || `Report ${r.id}` }, run: () => exportReport(r.id) })}><Download aria-hidden="true" /></Button></Can>
          {canDelete(r) && <Button variant="ghost" size="icon" className="size-11 text-destructive" aria-label={`Delete ${rowName(r)}`} title="Delete" onClick={(e) => { opener.current = e.currentTarget; del.reset(); setTarget({ report: r, period: `${r.month} ${r.year}` }); }}><Trash2 aria-hidden="true" /></Button>}
        </>
      )}>

      {isPending && allowed ? (
        <TableSkeleton columns={6} />
      ) : error ? (
        <p role="alert" className="text-sm text-destructive">
          {error.message}
        </p>
      ) : data && data.data.length === 0 ? (
        <EmptyState
          title="No reports found"
          description="Try changing your filters or create a report for this period."
          action={user?.role === "user" && can("reports:write") ? { href: "/reports/new", label: "Create report" } : undefined}
        />
      ) : (
        data && (
          <>
            {meta && (
              <div className="flex items-center justify-between gap-4">
                <p role="status" className="text-sm text-muted-foreground">
                  Page {meta.page} of {meta.total_pages} ({meta.total} reports)
                </p>
                <div className="flex gap-2">
                  <Button variant="outline" disabled={meta.page <= 1} onClick={() => goToPage(meta.page - 1)}>
                    Previous
                  </Button>
                  <Button
                    variant="outline"
                    disabled={meta.page >= meta.total_pages}
                    onClick={() => goToPage(meta.page + 1)}
                  >
                    Next
                  </Button>
                </div>
              </div>
            )}
          </>
        )
      )}

      </ReportsTableCard>
      {preview && <ExcelPreviewDialog target={preview.target} onDownload={() => download.mutateAsync(preview).then(() => true, () => false)} onClose={() => setPreview(null)} />}
      {reportPreview !== null && <ReportPreviewDialog reportId={reportPreview} onClose={() => setReportPreview(null)} />}
      <AlertDialog open={metadataPreview !== null} onOpenChange={(open) => !open && setMetadataPreview(null)}>
        <AlertDialogContent onCloseAutoFocus={(e) => { e.preventDefault(); (opener.current?.isConnected ? opener.current : document.getElementById("main-content"))?.focus(); }}>
          <AlertDialogHeader><AlertDialogTitle>Report Preview</AlertDialogTitle><AlertDialogDescription>Submission metadata</AlertDialogDescription></AlertDialogHeader>
          <dl className="grid grid-cols-2 gap-3 text-sm">
            {[["Worker", metadataPreview?.worker], ["Month", metadataPreview?.month], ["Year", metadataPreview?.year], ["Area", metadataPreview?.area_assignment], ["Church", metadataPreview?.church_name], ["Status", metadataPreview?.completed ? "Approved" : "Pending"]].map(([label, value]) => <div key={label}><dt className="text-muted-foreground">{label}</dt><dd>{value || "—"}</dd></div>)}
          </dl>
          <AlertDialogFooter><AlertDialogCancel>Close</AlertDialogCancel></AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
      <AlertDialog open={unapproveTarget !== null} onOpenChange={(open) => !open && !busy.length && setUnapproveTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Unapprove Report</AlertDialogTitle>
            <AlertDialogDescription>Admin notes are required before unapproving this report.</AlertDialogDescription>
          </AlertDialogHeader>
          <Label htmlFor="reports-admin-notes">Admin Notes</Label>
          <Textarea id="reports-admin-notes" aria-label="Admin Notes" required value={adminNotes} onChange={(event) => setAdminNotes(event.target.value)} maxLength={5000} />
          <AlertDialogFooter>
            <AlertDialogCancel disabled={busy.length > 0}>Cancel</AlertDialogCancel>
            <Button disabled={busy.length > 0 || !adminNotes.trim()} aria-busy={busy.length > 0} onClick={() => { if (!unapproveTarget) return; const report = unapproveTarget; setUnapproveTarget(null); approve.mutate({ report, adminNotes: adminNotes.trim() }); }}>{busy.length > 0 ? <><Loader2 aria-hidden="true" className="motion-safe:animate-spin" />Unapproving…</> : "Unapprove"}</Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
      <AlertDialog open={target !== null} onOpenChange={(open) => !open && !deleting && setTarget(null)}>
        <AlertDialogContent
          onCloseAutoFocus={(e) => {
            e.preventDefault();
            // After a delete the row is gone; the main content is the next best place.
            (opener.current?.isConnected ? opener.current : document.getElementById("main-content"))?.focus();
          }}
        >
          <AlertDialogHeader>
            <AlertDialogTitle>Delete {target?.period}?</AlertDialogTitle>
            <AlertDialogDescription>You can file this month again afterwards.</AlertDialogDescription>
          </AlertDialogHeader>
          {deleteError && (
            <p role="alert" className="text-sm text-destructive">
              {deleteError}
            </p>
          )}
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleting}>Cancel</AlertDialogCancel>
            {/* A plain Button, not AlertDialogAction: that one closes the dialog on click, and a failed delete must keep it open. */}
            <Button variant="destructive" onClick={() => target && del.mutate(target.report)} disabled={deleting} aria-busy={deleting}>
              {deleting ? <><Loader2 aria-hidden="true" className="motion-safe:animate-spin" />Deleting…</> : "Delete"}
            </Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}

export function Reports() {
  return (
    <main id="main-content" tabIndex={-1} className={`${reportsTheme} mx-auto flex w-full max-w-7xl flex-1 flex-col gap-6 p-4 sm:p-6 focus:outline-none`}>
      <Suspense fallback={<TableSkeleton columns={6} rows={3} />}>
        <ReportsList />
      </Suspense>
    </main>
  );
}
