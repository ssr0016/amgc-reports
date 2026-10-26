"use client";

import { useQuery } from "@tanstack/react-query";
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { getReport, type Report } from "@/lib/reports";
import { METRIC_SECTIONS, WEEKS } from "@/lib/report-form";
import { Loading } from "@/components/loading";
import { ErrorRetry } from "@/components/error-retry";

const statusLabels = {
  pending: "Pending",
  approved: "Approved",
  needs_revision: "Needs Revision",
} as const;

// D-A5: average of weeks with values; dash when none.
export function calculateAverage(values: Report["metrics"][string] | undefined): string {
  if (!values) return "—";
  const weeks = WEEKS.map((w) => values[w]).filter((v): v is number => v != null);
  if (weeks.length === 0) return "—";
  const sum = weeks.reduce<number>((acc, v) => acc + v, 0);
  return (sum / weeks.length).toFixed(2);
}

export function ReportPreviewDialog({ reportId, onClose }: { reportId: number; onClose: () => void }) {
  const { data: report, error, isPending, refetch } = useQuery({
    queryKey: ["report", reportId],
    queryFn: () => getReport(reportId),
  });

  return (
    <AlertDialog open onOpenChange={(open) => !open && onClose()}>
      <AlertDialogContent className="max-w-6xl max-h-[90vh] overflow-y-auto">
        <AlertDialogHeader>
          <AlertDialogTitle>Report Preview</AlertDialogTitle>
          <AlertDialogDescription>
            {report ? `${report.month} ${report.year} — ${report.worker || "No worker"}` : "Loading report…"}
          </AlertDialogDescription>
        </AlertDialogHeader>

        {isPending ? (
          <div className="py-8">
            <Loading />
          </div>
        ) : error ? (
          <ErrorRetry message={error.message} onRetry={() => refetch()} />
        ) : report ? (
          <div className="flex flex-col gap-6">
            {/* Metadata */}
            <section className="grid gap-2 text-sm">
              <div className="grid grid-cols-[120px_1fr] gap-2">
                <span className="font-medium">Worker:</span>
                <span>{report.worker || "—"}</span>
              </div>
              {report.admin_notes && (
                <div role="alert" className="mt-2 rounded-md border border-red-200 bg-red-50 p-3 text-red-800">
                  <div className="font-medium">Admin Notes</div>
                  <div className="mt-1 whitespace-pre-wrap">{report.admin_notes}</div>
                </div>
              )}
              <div className="grid grid-cols-[120px_1fr] gap-2">
                <span className="font-medium">Area:</span>
                <span>{report.area_assignment || "—"}</span>
              </div>
              <div className="grid grid-cols-[120px_1fr] gap-2">
                <span className="font-medium">Church:</span>
                <span>{report.church_name || "—"}</span>
              </div>
              <div className="grid grid-cols-[120px_1fr] gap-2">
                <span className="font-medium">Month/Year:</span>
                <span>{report.month} {report.year}</span>
              </div>
              <div className="grid grid-cols-[120px_1fr] gap-2">
                <span className="font-medium">Status:</span>
                <span>{report.status ? statusLabels[report.status] : "—"}</span>
              </div>
            </section>

            {/* Metric Tables */}
            {METRIC_SECTIONS.map((section) => (
              <section key={section.title} className="flex flex-col gap-3">
                <h3 className="text-base font-medium">{section.title}</h3>
                <Table className="table-fixed">
                  <TableHeader>
                    <TableRow>
                      <TableHead className="w-48">Metric</TableHead>
                      {WEEKS.map((w) => (
                        <TableHead key={w} className="w-20">Week {w.slice(4)}</TableHead>
                      ))}
                      <TableHead className="w-20">Avg</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {section.metrics.map((m) => {
                      const values = report.metrics[m.key];
                      return (
                        <TableRow key={m.key}>
                          <TableCell className="font-medium">{m.label}</TableCell>
                          {WEEKS.map((w) => (
                            <TableCell key={w}>{values?.[w] ?? "—"}</TableCell>
                          ))}
                          <TableCell className="font-medium">{calculateAverage(values)}</TableCell>
                        </TableRow>
                      );
                    })}
                  </TableBody>
                </Table>
              </section>
            ))}
            <section aria-labelledby="preview-report-details" className="min-w-0 border-t pt-6">
              <h3 id="preview-report-details" className="text-base font-medium">
                Report Details
              </h3>
              <dl className="mt-3 grid gap-4 text-sm">
                {([
                  ["names", "Names of New Believers"],
                  ["narrative_report", "Narrative Report"],
                  ["challenges", "Challenges"],
                  ["prayer_request", "Prayer Request"],
                ] as const).map(([field, label]) => (
                  <div key={field} className="min-w-0">
                    <dt className="font-medium">{label}</dt>
                    <dd className="mt-1 whitespace-pre-wrap [overflow-wrap:anywhere]">
                      {report[field] || "—"}
                    </dd>
                  </div>
                ))}
              </dl>
            </section>
          </div>
        ) : null}

        <AlertDialogFooter>
          <AlertDialogCancel>Close</AlertDialogCancel>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
