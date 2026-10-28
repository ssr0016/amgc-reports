"use client";

import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogCancel,
} from "@/components/ui/alert-dialog";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { ErrorRetry } from "@/components/error-retry";
import { TableSkeleton } from "@/components/loading-skeletons";
import { monthLabel, type AttendanceAnalytics } from "@/lib/analytics";
import { listReports } from "@/lib/reports";
import { cellText } from "./analytics-compliance";
import { formatValue } from "./analytics-format";

// ChurchDetail is one church's period: its reporting months, every month's
// report status, its metric totals and, for readers of every report, its narratives.
export function ChurchDetail({ data, churchId, canReadReports, onClose }: { data: AttendanceAnalytics; churchId: number; canReadReports: boolean; onClose: () => void }) {
  const church = data.churches.find((c) => c.id === churchId);
  const row = data.compliance.find((r) => r.church_id === churchId);
  if (!church || !row) return null;
  return (
    <AlertDialog open onOpenChange={(open) => !open && onClose()}>
      <AlertDialogContent className="max-h-[95vh] max-w-5xl overflow-y-auto">
        <AlertDialogHeader className="place-items-start text-left">
          <AlertDialogTitle>{church.name}</AlertDialogTitle>
          <AlertDialogDescription>
            {data.year} · {church.workers || "No church account left"} · account {church.account_status} ·{" "}
            {church.needs_review ? "reporting start month not set" : `reports ${monthLabel(church.start_month)}${church.start_is_default ? " (organization start)" : ""} to ${church.end_month ? monthLabel(church.end_month) : "ongoing"}`}{church.active ? "" : " · inactive church"}
          </AlertDialogDescription>
        </AlertDialogHeader>

        <section aria-labelledby="church-history" className="flex flex-col gap-2">
          <h3 id="church-history" className="text-sm font-medium">Report history · {row.submitted} of {row.expected} expected submitted, {row.approved} approved</h3>
          <div className="overflow-x-auto rounded-lg border">
            <Table aria-label={`${church.name} report history`}>
              <TableHeader><TableRow><TableHead>Month</TableHead><TableHead>Status</TableHead><TableHead>Submitted</TableHead><TableHead>Report</TableHead></TableRow></TableHeader>
              <TableBody>
                {row.cells.map((cell) => {
                  const { text } = cellText(cell);
                  return (
                    <TableRow key={cell.month}>
                      <TableCell>{data.months[cell.month - data.months[0].month]?.name}</TableCell>
                      <TableCell>{text}{cell.duplicates ? ` (${cell.duplicates} other report(s) not counted)` : ""}</TableCell>
                      <TableCell>{cell.submitted_on ?? "—"}</TableCell>
                      <TableCell>{cell.report_id ? <Link className="underline underline-offset-4" href={`/reports/${cell.report_id}`}>Open report #{cell.report_id}</Link> : "—"}</TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          </div>
        </section>

        <section aria-labelledby="church-metrics" className="flex flex-col gap-2">
          <h3 id="church-metrics" className="text-sm font-medium">Ministry figures · counted months, {data.status === "approved" ? "approved reports" : "all submitted reports"}</h3>
          <div className="overflow-x-auto rounded-lg border">
            <Table aria-label={`${church.name} metrics`}>
              <TableHeader><TableRow><TableHead>Metric</TableHead><TableHead className="text-right">Total</TableHead><TableHead className="text-right">Recorded entries</TableHead><TableHead className="text-right">Avg per service</TableHead></TableRow></TableHeader>
              <TableBody>
                {data.metrics.map((m) => {
                  const v = m.churches.find((c) => c.church_id === churchId);
                  const total = v?.value ?? null;
                  const state = v?.state === "blank" ? "Left blank" : v?.state === "not_approved" ? "Not approved yet" : v?.state === "no_report" ? "No report" : undefined;
                  return (
                    <TableRow key={m.metric}>
                      <TableCell>{m.label}</TableCell>
                      <TableCell className="text-right tabular-nums">{state ?? formatValue(total, m.unit)}</TableCell>
                      <TableCell className="text-right tabular-nums">{v?.observations ?? 0}</TableCell>
                      <TableCell className="text-right tabular-nums">{m.unit === "people" && total != null && v?.observations ? formatValue(total / v.observations) : ""}</TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          </div>
        </section>

        {canReadReports ? <Narratives data={data} churchName={church.name} /> : <p className="text-sm text-muted-foreground">Narratives, challenges and prayer requests are shown to those who may read every report.</p>}

        <AlertDialogFooter><AlertDialogCancel>Close</AlertDialogCancel></AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}

// Narratives reads the church's reports for the year through the reports API,
// so its own permission checks apply; the analytics answer carries no text.
function Narratives({ data, churchName }: { data: AttendanceAnalytics; churchName: string }) {
  const months = new Set(data.months.map((m) => m.name));
  const { data: page, error, isPending, refetch } = useQuery({
    queryKey: ["church-narratives", churchName, data.year],
    queryFn: () => listReports({ church: churchName, year: data.year, limit: 100 }),
  });
  const key = churchName.trim().toLowerCase();
  // The list filter matches part of a name; keep this church's own reports, in the period.
  const reports = (page?.data ?? [])
    .filter((r) => (r.church_name ?? "").trim().toLowerCase() === key && months.has(r.month))
    .sort((a, b) => data.months.findIndex((m) => m.name === a.month) - data.months.findIndex((m) => m.name === b.month));
  return (
    <section aria-labelledby="church-narratives" className="flex flex-col gap-2">
      <h3 id="church-narratives" className="text-sm font-medium">Narratives, challenges and prayer requests</h3>
      {isPending ? <TableSkeleton columns={3} rows={3} /> : error ? <ErrorRetry message={error.message} onRetry={() => refetch()} /> : reports.length === 0 ? (
        <p className="text-sm text-muted-foreground">No reports in this period.</p>
      ) : (
        <ul className="flex flex-col gap-3">
          {reports.map((r) => (
            <li key={r.id} className="rounded-lg border p-3 text-sm">
              <p className="font-medium">{r.month} {r.year} · <Link className="underline underline-offset-4" href={`/reports/${r.id}`}>report #{r.id}</Link></p>
              <dl className="mt-2 grid gap-2">
                {[["Narrative report", r.narrative_report], ["Challenges/problems encountered", r.challenges], ["Prayer requests", r.prayer_request]].map(([label, text]) => (
                  <div key={label}><dt className="text-xs font-medium text-muted-foreground">{label}</dt><dd className="whitespace-pre-wrap">{text || "—"}</dd></div>
                ))}
              </dl>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
