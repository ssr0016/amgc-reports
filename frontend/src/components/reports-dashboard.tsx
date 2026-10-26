"use client";

import { useQuery } from "@tanstack/react-query";
import { useState, type ReactNode } from "react";
import { Avatar } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
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

export function ReportsTableCard({ rows, scope, filters, children, actions, selectable, bulkActions }: {
  rows: Report[];
  scope: string;
  filters: ReactNode;
  children: ReactNode;
  actions: (report: Report) => ReactNode;
  selectable: boolean;
  bulkActions?: {
    busy?: boolean;
    approve: (reports: Report[]) => Promise<void>;
    unapprove: (reports: Report[]) => Promise<void>;
  };
}) {
  const [status, setStatus] = useState("all");
  const [selection, setSelection] = useState<{ scope: string; ids: number[] }>({ scope: "", ids: [] });
  const visible = rows.filter((r) => status === "all" || (r.status ?? (r.completed ? "approved" : "pending")) === status);
  const selectionScope = `${scope}:${status}`;
  const selected = selection.scope === selectionScope ? selection.ids.filter((id) => visible.some((r) => r.id === id)) : [];
  const selectedRows = visible.filter((r) => selected.includes(r.id));
  const allSelected = visible.length > 0 && selected.length === visible.length;
  function select(ids: number[]) { setSelection({ scope: selectionScope, ids }); }
  async function runBulk(action: "approve" | "unapprove") {
    if (!bulkActions) return;
    await bulkActions[action](selectedRows);
    select([]);
  }

  return (
    <Card className="gap-0 overflow-hidden rounded-lg py-0 shadow-none">
      <div className="border-b p-4">{filters}</div>
      <div className="flex flex-wrap items-center justify-between gap-2 border-b px-4 py-2">
        <div role="group" aria-label="Filter status on this page" className="flex gap-1">
          {["all", "approved", "pending", "needs_revision"].map((value) => (
            <Button key={value} variant={status === value ? "secondary" : "ghost"} className="min-h-11" aria-pressed={status === value} onClick={() => { setStatus(value); select([]); }}>
              {value === "needs_revision" ? "Needs Revision" : value[0].toUpperCase() + value.slice(1)}
            </Button>
          ))}
        </div>
        <p className="text-xs text-muted-foreground">Status filters apply to this page</p>
      </div>
      <Table>
        <TableHeader>
          <TableRow>
            {selectable && <TableHead className="w-12"><label className="flex size-11 items-center justify-center"><input type="checkbox" className="size-4 accent-[var(--primary)]" aria-label="Select all visible reports" checked={allSelected} ref={(node) => { if (node) node.indeterminate = selected.length > 0 && !allSelected; }} onChange={() => select(allSelected ? [] : visible.map((r) => r.id))} /></label></TableHead>}
            <TableHead>Worker</TableHead><TableHead>Month</TableHead><TableHead className="text-right">Year</TableHead>
            <TableHead>Area</TableHead><TableHead>Church</TableHead><TableHead>Status</TableHead><TableHead className="text-right">Actions</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {visible.map((r) => (
            <TableRow key={r.id} data-state={selected.includes(r.id) ? "selected" : undefined}>
              {selectable && <TableCell><label className="flex size-11 items-center justify-center"><input type="checkbox" className="size-4 accent-[var(--primary)]" aria-label={`Select ${r.worker || "report"}, ${r.month} ${r.year}`} checked={selected.includes(r.id)} onChange={() => select(selected.includes(r.id) ? selected.filter((id) => id !== r.id) : [...selected, r.id])} /></label></TableCell>}
              <TableCell><div className="flex items-center gap-3"><Avatar name={r.worker || "Unknown worker"} className="bg-secondary! text-secondary-foreground! cursor-default" /><div className="flex min-w-36 flex-col items-start gap-1"><span className="font-medium">{r.worker || "—"}</span><ReportOwnerBadges report={r} /></div></div></TableCell>
              <TableCell>{r.month}</TableCell><TableCell className="text-right font-mono tabular-nums">{r.year}</TableCell>
              <TableCell>{r.area_assignment || "—"}</TableCell><TableCell>{r.church_name || "—"}</TableCell>
              <TableCell>
                <div className="flex flex-wrap gap-1">
                  <Badge className={(r.status ?? (r.completed ? "approved" : "pending")) === "approved" ? "border-transparent bg-[var(--approved)]/10 text-[var(--approved)]" : (r.status ?? "pending") === "needs_revision" ? "border-transparent bg-red-100 text-red-700" : "border-transparent bg-[var(--pending)]/10 text-[var(--pending)]"}>{(r.status ?? (r.completed ? "approved" : "pending")) === "needs_revision" ? "Needs Revision" : (r.status ?? (r.completed ? "approved" : "pending")) === "approved" ? "Approved" : "Pending"}</Badge>
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
      {selectable && selected.length > 0 && (
        <aside aria-label="Bulk Actions" className="sticky bottom-4 z-10 mx-4 mb-4 flex flex-wrap items-center justify-between gap-3 rounded-lg border bg-popover p-4 shadow-lg">
          <p role="status" className="text-sm"><span className="font-mono tabular-nums">{selected.length}</span> selected</p>
          {bulkActions && (
            <div className="flex flex-wrap gap-2">
              <Button variant="outline" disabled={bulkActions.busy} aria-busy={bulkActions.busy} onClick={() => void runBulk("approve")}>Bulk Approve</Button>
              <Button variant="outline" disabled={bulkActions.busy} aria-busy={bulkActions.busy} onClick={() => void runBulk("unapprove")}>Bulk Unapprove</Button>
            </div>
          )}
          <Button variant="outline" disabled={bulkActions?.busy} onClick={() => select([])}>Clear selection</Button>
        </aside>
      )}
    </Card>
  );
}
