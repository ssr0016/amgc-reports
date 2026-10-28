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
import { WEEKS } from "@/lib/report-form";
import { TableSkeleton } from "@/components/loading-skeletons";
import { ErrorRetry } from "@/components/error-retry";
import { ReportStatusBadge } from "@/components/report-status-badge";

const excelMetricRows = [
  { key: "worship_service", label: "Worship Service" },
  { key: "sunday_school", label: "Sunday School" },
  { key: "prayer_meeting", label: "Prayer Meeting" },
  { key: "bible_studies", label: "Bible Studies" },
  { key: "mens_fellowship", label: "Mens Fellowships" },
  { key: "womens_fellowship", label: "Womens Fellowships" },
  { key: "youth_fellowship", label: "Youth Fellowships" },
  { key: "children_fellowship", label: "Children Fellowships" },
  { key: "outreach", label: "Outreach" },
  { key: "baptism", label: "Baptism" },
  { key: "training", label: "Training/ Seminars" },
  { key: "leadership", label: "Leadership Conference" },
  { key: "other", label: "Other" },
  { key: "family_day", label: "Family Day" },
  { key: "tithes_offering", label: "Tithes & Offering" },
] as const;

const excelOtherRows = [
  { key: "home_visited", label: "Home Visited" },
  { key: "bible_study_group_led", label: "Bible Study Group Led" },
  { key: "sermon_preached", label: "Sermon/Message Preached" },
  { key: "person_newly_contacted", label: "Person Newly Contacted" },
  { key: "person_followed_up", label: "Person Followed-up" },
  { key: "person_evangelized", label: "Person Led To Christ" },
] as const;

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
      <AlertDialogContent className="max-h-[95vh] max-w-7xl overflow-y-auto bg-white text-black dark:bg-white dark:text-black">
        <AlertDialogHeader className="-mx-4 -mt-4 place-items-start rounded-t-xl bg-black px-4 py-3 text-left text-white sm:-mx-6 sm:-mt-6 sm:px-6">
          <AlertDialogTitle className="text-base font-bold tracking-tight">
            AMGCReports — Report Preview
          </AlertDialogTitle>
          <AlertDialogDescription className="text-xs text-slate-300">
            {report ? `${report.month} ${report.year} — ${report.worker || "No worker"}` : "Report preview"}
          </AlertDialogDescription>
        </AlertDialogHeader>

        {isPending ? (
          <div className="py-8">
            <TableSkeleton columns={4} rows={6} />
          </div>
        ) : error ? (
          <ErrorRetry message={error.message} onRetry={() => refetch()} />
        ) : report ? (
          <div className="mx-auto w-full max-w-[1100px] bg-white font-sans text-[11px] text-black">
            <header className="border-b border-black pb-3 text-center">
              {/* Static public asset; Next image optimization is unnecessary for this fixed report logo. */}
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img src="/amgc-logo.png" alt="AMGC logo" className="mx-auto mb-2 h-16 w-auto object-contain" />
              <h1 className="text-lg font-bold">ANG MANANAMPALATAYANG GUMAWA</h1>
              <h2 className="text-sm font-bold">NATIONAL WORKERS&apos; MONTHLY MINISTRY REPORT</h2>
              <p className="mt-1 text-xs">Month of: {report.month} {report.year}</p>
            </header>

            <section className="grid grid-cols-1 border-x border-b border-black sm:grid-cols-2">
              {[
                ["Worker:", report.worker],
                ["Area of Assignment:", report.area_assignment],
                ["Name of Church/Outreach:", report.church_name],
                ["Status:", report.status ? <ReportStatusBadge status={report.status} /> : "—"],
              ].map(([label, value], index) => (
                <div key={index} className="grid grid-cols-[minmax(10rem,auto)_1fr] border-b border-black px-2 py-1 last:border-b-0 sm:odd:border-r">
                  <span className="font-bold">{label}</span>
                  <span>{value || "—"}</span>
                </div>
              ))}
            </section>

            <section className="mt-4">
              <ReportMetricTable title="Activities" rows={excelMetricRows} report={report} />
              <ReportMetricTable title="Other Statistics" rows={excelOtherRows} report={report} names={report.names} />
            </section>

            <section aria-labelledby="preview-report-details" className="mt-4 border border-black">
              <h3 id="preview-report-details" className="border-b border-black bg-black px-2 py-1 font-bold text-white">
                Report Details
              </h3>
              <dl className="grid gap-0">
                {([
                  ["narrative_report", "Narrative Report:"],
                  ["challenges", "Challenges/Problem Encountered:"],
                  ["prayer_request", "Prayer Request:"],
                ] as const).map(([field, label], index) => (
                  <div key={field} className={`grid grid-cols-[minmax(12rem,18%)_1fr] border-b border-black last:border-b-0 ${index % 2 ? "bg-slate-50" : "bg-white"}`}>
                    <dt className="border-r border-black px-2 py-1 font-bold">{label}</dt>
                    <dd className="whitespace-pre-wrap px-2 py-1 [overflow-wrap:anywhere]">{report[field] || "—"}</dd>
                  </div>
                ))}
              </dl>
            </section>

            <div className="mt-4 grid gap-2 text-sm">
              {report.admin_notes && (
                <div role="alert" className="rounded-md border border-red-200 bg-red-50 p-3 text-red-800">
                  <div className="font-medium">Admin Notes</div>
                  <div className="mt-1 whitespace-pre-wrap">{report.admin_notes}</div>
                </div>
              )}
              {report.user_notes && (
                <div role="alert" className="rounded-md border border-blue-200 bg-blue-50 p-3 text-blue-800">
                  <div className="font-medium">User Notes</div>
                  <div className="mt-1 whitespace-pre-wrap">{report.user_notes}</div>
                </div>
              )}
            </div>
          </div>
        ) : null}

        <AlertDialogFooter>
          <AlertDialogCancel className="min-w-24 border-slate-700 bg-white px-4 text-slate-900 hover:bg-slate-100 hover:text-slate-950">
            Close
          </AlertDialogCancel>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}

function ReportMetricTable({
  title,
  rows,
  report,
  names,
}: {
  title: string;
  rows: readonly { key: string; label: string }[];
  report: Report;
  // Excel puts Names after the last statistic: a label row, then the names below it.
  names?: string;
}) {
  const zebra = (index: number) => `border-black ${index % 2 ? "bg-slate-50 hover:bg-slate-100" : "bg-white hover:bg-slate-100"}`;
  return (
    <div className="mb-3 overflow-x-auto border border-black last:mb-0">
      <div className="bg-black px-2 py-1 font-bold text-white">{title}</div>
      <Table aria-label={`${title} metrics`} className="table-fixed border-collapse text-[11px] tabular-nums">
        <TableHeader className="bg-[#E0E0E0] shadow-none">
          <TableRow className="border-black bg-[#E0E0E0] hover:bg-[#E0E0E0]">
            <TableHead className="w-[30%] border-r border-black px-2 py-1 font-bold text-black h-10 normal-case tracking-normal text-[11px]">Activities</TableHead>
            {WEEKS.map((w) => <TableHead key={w} className="border-r border-black px-2 py-1 text-center font-bold text-black h-10 normal-case tracking-normal text-[11px]">Week {w.slice(4)}</TableHead>)}
            <TableHead className="px-2 py-1 text-center font-bold text-black h-10 normal-case tracking-normal text-[11px]">Average</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map((row, index) => {
            const values = report.metrics[row.key];
            return (
              <TableRow key={row.key} className={zebra(index)}>
                <TableCell className="border-r border-black px-2 py-1 font-medium text-black">{row.label}</TableCell>
                {WEEKS.map((w) => <TableCell key={w} className="border-r border-black px-2 py-1 text-center text-black">{values?.[w] ?? "—"}</TableCell>)}
                <TableCell className="px-2 py-1 text-center font-medium text-black">{calculateAverage(values)}</TableCell>
              </TableRow>
            );
          })}
          {names !== undefined && (
            <>
              <TableRow className={zebra(rows.length)}>
                <TableCell className="border-r border-black px-2 py-1 font-bold text-black">Names:</TableCell>
                {WEEKS.map((w) => <TableCell key={w} className="border-r border-black px-2 py-1" />)}
                <TableCell className="px-2 py-1" />
              </TableRow>
              <TableRow className={zebra(rows.length + 1)}>
                <TableCell colSpan={WEEKS.length + 2} className="whitespace-pre-wrap px-2 py-1 text-black [overflow-wrap:anywhere]">{names || "—"}</TableCell>
              </TableRow>
            </>
          )}
        </TableBody>
      </Table>
    </div>
  );
}
