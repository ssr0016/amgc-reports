"use client";

import { useQuery } from "@tanstack/react-query";
import { useState, type ReactNode } from "react";
import { Avatar } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
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
export { dashboardTheme as reportsTheme } from "./dashboard-theme";

export function ReportsHeader({ title, heading: Heading = "h2", children }: { title: string; heading?: "h1" | "h2"; children?: ReactNode }) {
  return (
    <header className="flex flex-wrap items-start justify-between gap-4">
      <div>
        <Heading className="text-2xl font-medium tracking-tight">{title}</Heading>
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
  const cards = [
    { label: "Total Reports", value: data?.meta.total, note: "Matching current filters" },
    { label: "Approved", value: approved, note: "On this page", tone: "text-[var(--approved)]" },
    { label: "Pending", value: data ? data.data.filter((r) => (r.status ?? (r.completed ? "approved" : "pending")) === "pending").length : undefined, note: "On this page", tone: "text-[var(--pending)]" },
    { label: "Needs Revision", value: allowed ? needsRevision : undefined, note: allowed ? (stats.isError ? "Count unavailable" : "All reports this month") : "Admin access required", tone: "text-red-700" },
  ];
  return (
    <section aria-label="Report statistics" className="grid grid-cols-2 gap-4 xl:grid-cols-4">
      {cards.map((c) => (
        <Card key={c.label} className="gap-2 rounded-lg p-4 shadow-none">
          <p className="text-sm text-muted-foreground">{c.label}</p>
          <p className={`font-mono text-3xl tabular-nums ${c.tone ?? ""}`}>{c.value?.toLocaleString() ?? "—"}</p>
          <p className="text-xs text-muted-foreground">{c.note}</p>
          <p className="text-xs text-muted-foreground">Last-month comparison unavailable</p>
        </Card>
      ))}
    </section>
  );
}

export function ReportsTableCard({ rows, filters, children, actions, selectable, bulkActions }: {
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
}) {
  const [status, setStatus] = useState("all");
  const [selectedReports, setSelectedReports] = useState<Report[]>([]);
  const [bulkUnapproveOpen, setBulkUnapproveOpen] = useState(false);
  const [bulkAdminNotes, setBulkAdminNotes] = useState("");
  const reportStatus = (r: Report) => r.status ?? (r.completed ? "approved" : "pending");
  const visible = rows.filter((r) => status === "all" || reportStatus(r) === status);
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
      <div className="border-b p-4">{filters}</div>
      <div className="flex flex-wrap items-center justify-between gap-2 border-b px-4 py-2">
        <div role="group" aria-label="Filter status on this page" className="flex gap-1">
          {["all", "draft", "approved", "pending", "needs_revision"].map((value) => (
            <Button key={value} variant={status === value ? "secondary" : "ghost"} className="min-h-11" aria-pressed={status === value} onClick={() => setStatus(value)}>
              {value === "needs_revision" ? "Needs Revision" : value[0].toUpperCase() + value.slice(1)}
            </Button>
          ))}
        </div>
        <p className="text-xs text-muted-foreground">Status filters apply to this page</p>
      </div>
      <Table>
        <TableHeader>
          <TableRow>
            {selectable && <TableHead className="w-12"><label className="flex size-11 items-center justify-center"><input type="checkbox" className="size-4 accent-[var(--primary)]" aria-label="Select all visible reports" checked={allSelected} ref={(node) => { if (node) node.indeterminate = selectedVisible.length > 0 && !allSelected; }} onChange={toggleVisible} /></label></TableHead>}
            <TableHead>Worker</TableHead><TableHead>Month</TableHead><TableHead className="text-right">Year</TableHead>
            <TableHead>Area</TableHead><TableHead>Church</TableHead><TableHead>Status</TableHead><TableHead className="text-right">Actions</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {visible.map((r) => (
            <TableRow key={r.id} data-state={selectedIds.has(r.id) ? "selected" : undefined}>
              {selectable && <TableCell><label className="flex size-11 items-center justify-center"><input type="checkbox" className="size-4 accent-[var(--primary)]" aria-label={`Select ${r.worker || "report"}, ${r.month} ${r.year}`} checked={selectedIds.has(r.id)} onChange={() => toggleReport(r)} /></label></TableCell>}
              <TableCell><div className="flex items-center gap-3"><Avatar name={r.worker || "Unknown worker"} className="bg-secondary! text-secondary-foreground! cursor-default" /><div className="flex min-w-36 flex-col items-start gap-1"><span className="font-medium">{r.worker || "—"}</span><ReportOwnerBadges report={r} /></div></div></TableCell>
              <TableCell>{r.month}</TableCell><TableCell className="text-right font-mono tabular-nums">{r.year}</TableCell>
              <TableCell>{r.area_assignment || "—"}</TableCell><TableCell>{r.church_name || "—"}</TableCell>
              <TableCell>
                <div className="flex flex-wrap gap-1">
                  <Badge className={reportStatus(r) === "approved" ? "border-transparent bg-[var(--approved)]/10 text-[var(--approved)]" : reportStatus(r) === "needs_revision" ? "border-transparent bg-red-100 text-red-700" : reportStatus(r) === "draft" ? "border-transparent bg-slate-100 text-slate-700" : "border-transparent bg-[var(--pending)]/10 text-[var(--pending)]"}>{reportStatus(r) === "needs_revision" ? "Needs Revision" : reportStatus(r) === "approved" ? "Approved" : reportStatus(r) === "draft" ? "Draft" : "Pending"}</Badge>
                  {r.status === "pending" && r.last_revised_at && <Badge variant="outline">Revised</Badge>}
                </div>
              </TableCell>
              <TableCell><div className="flex justify-end gap-2">{actions(r)}</div></TableCell>
            </TableRow>
          ))}
          {rows.length > 0 && visible.length === 0 && <TableRow><TableCell colSpan={selectable ? 8 : 7} className="py-10 text-center"><p role="status">No {status} reports on this page.</p></TableCell></TableRow>}
        </TableBody>
      </Table>
      <div className="border-t p-4">{children}</div>
      {selectable && selectedReports.length > 0 && (
        <aside aria-label="Bulk Actions" className="sticky bottom-4 z-10 mx-4 mb-4 flex flex-wrap items-center justify-between gap-3 rounded-lg border bg-popover p-4 shadow-lg">
          <p role="status" className="text-sm"><span className="font-mono tabular-nums">{selectedReports.length}</span> selected</p>
          {bulkActions && (
            <div className="flex flex-wrap gap-2">
              <Button variant="outline" disabled={bulkActions.busy} aria-busy={bulkActions.busy} onClick={() => void runBulk("approve")}>Bulk Approve</Button>
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
            <Button disabled={bulkActions?.busy || !bulkAdminNotes.trim()} onClick={() => { setBulkUnapproveOpen(false); void runBulk("unapprove", bulkAdminNotes.trim()); }}>Bulk Unapprove</Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </Card>
  );
}
