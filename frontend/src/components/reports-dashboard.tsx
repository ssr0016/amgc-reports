"use client";

import { useQuery } from "@tanstack/react-query";
import { useState, type ReactNode } from "react";
import { Avatar } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { ReportStatusBadge } from "@/components/report-status-badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Textarea } from "@/components/ui/textarea";
import { AlertDialog, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/components/ui/alert-dialog";
import { ReportOwnerBadges } from "@/components/report-owner-badges";
import type { Page, Report } from "@/lib/reports";
import { useSession } from "@/lib/session";
import { getStats } from "@/lib/stats";
import { CircleCheck, Clock, FilePen, FileText, Filter, Loader2, RefreshCw, type LucideIcon } from "lucide-react";
import { StatCard, type StatTone } from "@/components/stat-card";

// Subtle row icon buttons: muted until the row is hovered, then each shows its colour; hovering the icon adds a tint.
export const iconButton = "size-11 rounded-md text-muted-foreground duration-150";
export const tone = {
  blue: "group-hover/row:text-blue-600 hover:bg-blue-500/10 dark:hover:bg-blue-500/10 hover:text-blue-600 dark:group-hover/row:text-blue-400 dark:hover:text-blue-400",
  amber: "group-hover/row:text-amber-600 hover:bg-amber-500/10 dark:hover:bg-amber-500/10 hover:text-amber-600 dark:group-hover/row:text-amber-400 dark:hover:text-amber-400",
  green: "group-hover/row:text-green-600 hover:bg-green-500/10 dark:hover:bg-green-500/10 hover:text-green-600 dark:group-hover/row:text-green-400 dark:hover:text-green-400",
  red: "group-hover/row:text-red-600 hover:bg-red-500/10 dark:hover:bg-red-500/10 hover:text-red-600 dark:group-hover/row:text-red-400 dark:hover:text-red-400",
};
export { dashboardTheme as reportsTheme } from "./dashboard-theme";

// Resubmitted after a revision request, whatever its status now.
const isRevised = (r: Report) => !!r.last_revised_at;
const revisedOn = new Intl.DateTimeFormat("en-US", { timeZone: "Asia/Manila", month: "short", day: "numeric", year: "numeric" });

export function ReportsHeader({ title, heading: Heading = "h2", children }: { title: string; heading?: "h1" | "h2"; children?: ReactNode }) {
  return (
    <header className="flex flex-wrap items-start justify-between gap-4">
      <div>
        <Heading className="text-2xl font-semibold tracking-tight">{title}</Heading>
        <p className="mt-2 text-sm text-muted-foreground">Monthly ministry reports from workers across AMGC churches.</p>
      </div>
      <div className="flex flex-wrap items-center gap-2">
        {children}
      </div>
    </header>
  );
}

export function ReportsStats({ data }: { data?: Page<Report> }) {
  const { can } = useSession();
  const allowed = can("tracker:read");
  const stats = useQuery({ queryKey: ["admin-stats"], queryFn: getStats, enabled: allowed, staleTime: 30_000 });
  const approved = data?.data.filter((r) => (r.status ?? (r.completed ? "approved" : "pending")) === "approved").length;
  const needsRevision = stats.data?.reports_needs_revision;
  const cards: { label: string; value?: number; note: string; icon: LucideIcon; tone: StatTone }[] = [
    { label: "Total Reports", value: data?.meta.total, note: "Matching current filters", icon: FileText, tone: "neutral" },
    { label: "Approved", value: approved, note: "On this page", icon: CircleCheck, tone: "green" },
    { label: "Pending", value: data ? data.data.filter((r) => (r.status ?? (r.completed ? "approved" : "pending")) === "pending").length : undefined, note: "On this page", icon: Clock, tone: "amber" },
    { label: "Needs Revision", value: allowed ? needsRevision : undefined, note: allowed ? (stats.isError ? "Count unavailable" : "All reports this month") : "Admin access required", icon: FilePen, tone: "red" },
  ];
  return (
    <section aria-label="Report statistics" className="grid grid-cols-2 gap-4 lg:grid-cols-4 lg:gap-5">
      {cards.map((c) => (
        <StatCard
          key={c.label}
          label={c.label}
          value={c.value?.toLocaleString() ?? "—"}
          icon={c.icon}
          tone={c.tone}
          note={<><p>{c.note}</p><p className="text-xs">Last-month comparison unavailable</p></>}
        />
      ))}
    </section>
  );
}

export function ReportsTableCard({ rows, filters, children, actions, selectable, bulkActions, onRevisedChange }: {
  rows: Report[];
  filters: ReactNode;
  children: ReactNode;
  actions: (report: Report) => ReactNode;
  selectable: boolean;
  bulkActions?: {
    busy?: boolean;
    approve: (reports: Report[]) => Promise<void>;
    unapprove: (reports: Report[], adminNotes: string) => Promise<void>;
  };
  /** The Revised tab asks the server for every revised report, so the page refetches. */
  onRevisedChange: (revised: boolean) => void;
}) {
  const [status, setStatus] = useState("all");
  const [selectedReports, setSelectedReports] = useState<Report[]>([]);
  const [bulkUnapproveOpen, setBulkUnapproveOpen] = useState(false);
  const [bulkAdminNotes, setBulkAdminNotes] = useState("");
  const reportStatus = (r: Report) => r.status ?? (r.completed ? "approved" : "pending");
  const visible = rows.filter((r) => status === "all" || (status === "revised" ? isRevised(r) : reportStatus(r) === status));
  function pickStatus(value: string) {
    if ((value === "revised") !== (status === "revised")) onRevisedChange(value === "revised");
    setStatus(value);
  }
  const selectedIds = new Set(selectedReports.map((r) => r.id));
  const selectedVisible = visible.filter((r) => selectedIds.has(r.id));
  const allSelected = visible.length > 0 && selectedVisible.length === visible.length;
  function toggleReport(report: Report) {
    setSelectedReports((current) => current.some((r) => r.id === report.id) ? current.filter((r) => r.id !== report.id) : [...current, report]);
  }
  function toggleVisible() {
    setSelectedReports((current) => {
      const visibleIds = new Set(visible.map((r) => r.id));
      if (allSelected) return current.filter((r) => !visibleIds.has(r.id));
      const selected = new Map(current.map((r) => [r.id, r]));
      visible.forEach((r) => selected.set(r.id, r));
      return [...selected.values()];
    });
  }
  async function runBulk(action: "approve" | "unapprove", adminNotes = "") {
    if (!bulkActions) return;
    if (action === "approve") await bulkActions.approve(selectedReports);
    else await bulkActions.unapprove(selectedReports, adminNotes);
    setSelectedReports([]);
  }

  return (
    <Card className="gap-0 overflow-hidden rounded-lg py-0 shadow-none">
      <div className="flex flex-col gap-5 border-b p-5 sm:p-6">
        <p className="flex items-center gap-2 text-sm font-medium text-muted-foreground">
          <Filter aria-hidden="true" className="size-4" />
          Filter reports
        </p>
        {filters}
      </div>
      <div className="flex flex-wrap items-center justify-between gap-3 border-b px-5 py-3 sm:px-6">
        {/* A segmented control: the pressed tab is lifted onto the card colour. */}
        <div role="group" aria-label="Filter status on this page" className="flex max-w-full gap-1 overflow-x-auto rounded-lg bg-muted p-1">
          {["all", "draft", "approved", "pending", "needs_revision", "revised"].map((value) => (
            <Button key={value} variant="ghost" className="rounded-md px-3 text-muted-foreground hover:bg-background/60 aria-pressed:bg-background aria-pressed:text-foreground aria-pressed:shadow-sm dark:hover:bg-background/40" aria-pressed={status === value} onClick={() => pickStatus(value)}>
              {value === "needs_revision" ? "Needs Revision" : value[0].toUpperCase() + value.slice(1)}
            </Button>
          ))}
        </div>
        <p className="text-xs text-muted-foreground">{status === "revised" ? "Showing every revised report" : "Status filters apply to this page"}</p>
      </div>
      {/* The container must scroll vertically for the header to stick, as on the Logs page. */}
      <div className="min-w-0 p-5 sm:p-6 [&_[data-slot=table-container]]:max-h-[70vh]">
      <Table>
        <TableHeader className="sticky top-0 z-10 bg-muted shadow-[0_1px_0_var(--border)] [&_th]:h-11 [&_th]:text-xs [&_th]:tracking-wide [&_th]:text-muted-foreground [&_th]:uppercase">
          <TableRow>
            {selectable && <TableHead className="w-12"><label className="flex size-11 items-center justify-center"><input type="checkbox" className="size-4 accent-[var(--primary)]" aria-label="Select all visible reports" checked={allSelected} ref={(node) => { if (node) node.indeterminate = selectedVisible.length > 0 && !allSelected; }} onChange={toggleVisible} /></label></TableHead>}
            <TableHead>Worker</TableHead><TableHead>Month</TableHead><TableHead className="text-right">Year</TableHead>
            <TableHead>Area</TableHead><TableHead>Church</TableHead><TableHead>Status</TableHead><TableHead className="text-right">Actions</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {visible.map((r) => (
            <TableRow key={r.id} className="group/row" data-state={selectedIds.has(r.id) ? "selected" : undefined}>
              {selectable && <TableCell><label className="flex size-11 items-center justify-center"><input type="checkbox" className="size-4 accent-[var(--primary)]" aria-label={`Select ${r.worker || "report"}, ${r.month} ${r.year}`} checked={selectedIds.has(r.id)} onChange={() => toggleReport(r)} /></label></TableCell>}
              <TableCell><div className="flex items-center gap-3"><Avatar name={r.worker || "Unknown worker"} className="bg-secondary! text-secondary-foreground! cursor-default" /><div className="flex min-w-36 flex-col items-start gap-1"><span className="font-medium">{r.worker || "—"}</span><ReportOwnerBadges report={r} /></div></div></TableCell>
              <TableCell>{r.month}</TableCell><TableCell className="text-right font-mono tabular-nums">{r.year}</TableCell>
              <TableCell>{r.area_assignment || "—"}</TableCell><TableCell>{r.church_name || "—"}</TableCell>
              <TableCell>
                <div className="flex flex-col items-start gap-1">
                  <div className="flex flex-wrap gap-1">
                    <ReportStatusBadge status={reportStatus(r)} />
                    {isRevised(r) && (
                      <Badge className="gap-1 bg-violet-50 font-semibold text-violet-800 ring-1 ring-violet-600/30 ring-inset dark:bg-violet-500/15 dark:text-violet-200 dark:ring-violet-400/40">
                        <RefreshCw aria-hidden="true" className="size-3" />Revised
                      </Badge>
                    )}
                  </div>
                  {isRevised(r) && <time dateTime={r.last_revised_at} className="text-xs text-muted-foreground">Revised on {revisedOn.format(new Date(r.last_revised_at!))}</time>}
                </div>
              </TableCell>
              <TableCell><div className="flex justify-end gap-1">{actions(r)}</div></TableCell>
            </TableRow>
          ))}
          {rows.length > 0 && visible.length === 0 && <TableRow><TableCell colSpan={selectable ? 8 : 7} className="py-10 text-center"><p role="status">No {status} reports on this page.</p></TableCell></TableRow>}
        </TableBody>
      </Table>
      {/* Pagination brings its own top rule and spacing. */}
      <div className="mt-4 [&>nav]:mt-0">{children}</div>
      </div>
      {selectable && selectedReports.length > 0 && (
        <aside aria-label="Bulk Actions" className="sticky bottom-4 z-10 mx-4 mb-4 flex flex-wrap items-center justify-between gap-3 rounded-lg border bg-popover p-4 shadow-lg">
          <p role="status" className="text-sm"><span className="font-mono tabular-nums">{selectedReports.length}</span> selected</p>
          {bulkActions && (
            <div className="flex flex-wrap gap-2">
              <Button variant="outline" disabled={bulkActions.busy} aria-busy={bulkActions.busy} onClick={() => void runBulk("approve")}>{bulkActions.busy ? <><Loader2 aria-hidden="true" className="motion-safe:animate-spin" />Updating…</> : "Bulk Approve"}</Button>
              <Button variant="outline" disabled={bulkActions.busy} aria-busy={bulkActions.busy} onClick={() => { setBulkAdminNotes(""); setBulkUnapproveOpen(true); }}>Bulk Unapprove</Button>
            </div>
          )}
          <Button variant="outline" disabled={bulkActions?.busy} onClick={() => setSelectedReports([])}>Clear selection</Button>
        </aside>
      )}
      <AlertDialog open={bulkUnapproveOpen} onOpenChange={setBulkUnapproveOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Bulk Unapprove Reports</AlertDialogTitle>
            <AlertDialogDescription>Add admin notes explaining why these reports need attention.</AlertDialogDescription>
          </AlertDialogHeader>
          <Label htmlFor="bulk-admin-notes">Admin Notes</Label>
          <Textarea id="bulk-admin-notes" aria-label="Admin Notes" required value={bulkAdminNotes} onChange={(event) => setBulkAdminNotes(event.target.value)} maxLength={5000} />
          <AlertDialogFooter>
            <AlertDialogCancel disabled={bulkActions?.busy}>Cancel</AlertDialogCancel>
            <Button disabled={bulkActions?.busy || !bulkAdminNotes.trim()} aria-busy={bulkActions?.busy} onClick={() => { setBulkUnapproveOpen(false); void runBulk("unapprove", bulkAdminNotes.trim()); }}>{bulkActions?.busy ? <><Loader2 aria-hidden="true" className="motion-safe:animate-spin" />Updating…</> : "Bulk Unapprove"}</Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </Card>
  );
}
