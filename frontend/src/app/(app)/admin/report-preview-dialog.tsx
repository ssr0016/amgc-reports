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
                <span>{report.completed ? "Completed" : "Pending"}</span>
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
          </div>
        ) : null}

        <AlertDialogFooter>
          <AlertDialogCancel>Close</AlertDialogCancel>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
