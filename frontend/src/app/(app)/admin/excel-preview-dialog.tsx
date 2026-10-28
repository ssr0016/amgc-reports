"use client";

import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { Loader2 } from "lucide-react";
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
import { Label } from "@/components/ui/label";
import { NativeSelect } from "@/components/ui/native-select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { ApiError } from "@/lib/api";
import { retryText } from "@/lib/auth-form";
import { getReport, previewExcel } from "@/lib/reports";
import { TableSkeleton } from "@/components/loading-skeletons";
import { ErrorRetry } from "@/components/error-retry";
import { ReportStatusBadge } from "@/components/report-status-badge";

// One report (the row's Excel button) or one month (Download Bulk).
export type PreviewTarget = { reportId: number; label: string } | { month: string; year: number };

// Excel status tones; dark: repeats them because this dialog stays white in the dark theme.
const EXCEL_TONE: Record<string, string> = {
  approved: "bg-green-100 text-green-800 dark:bg-green-100 dark:text-green-800",
  pending: "bg-amber-100 text-amber-800 dark:bg-amber-100 dark:text-amber-800",
  needs_revision: "bg-amber-100 text-amber-800 dark:bg-amber-100 dark:text-amber-800",
  no_report_submitted: "bg-red-100 text-red-800 dark:bg-red-100 dark:text-red-800",
};

function ExcelStatusBadge({ status }: { status: string }) {
  return <ReportStatusBadge status={status} className={EXCEL_TONE[status.toLowerCase().replaceAll(" ", "_")]} />;
}

// Shows the workbook an export would build before it is downloaded (D-A15). The
// preview writes no audit row; only Download does, through the real export.
// Mount it only while open: closing unmounts it, and Radix returns focus to the
// button that opened it.
export function ExcelPreviewDialog({
  target,
  onDownload,
  onClose,
}: {
  target: PreviewTarget;
  onDownload: () => Promise<boolean>;
  onClose: () => void;
}) {
  const single = "reportId" in target;
  const { data, error, isPending, refetch } = useQuery({
    queryKey: ["excel-preview", target],
    queryFn: async () => {
      const preview = await previewExcel(single ? { report_id: target.reportId } : { month: target.month, year: target.year });
      if (!single) return preview;
      const report = await getReport(target.reportId);
      return { ...preview, status: report.status };
    },
    // Every opening reads the current data.
    gcTime: 0,
    retry: false,
  });
  const [sheet, setSheet] = useState(0);
  const [downloading, setDownloading] = useState(false);
  const shown = data?.sheets[sheet];
  const previewStatus = data && "status" in data ? data.status : undefined;

  async function download() {
    setDownloading(true);
    const ok = await onDownload();
    setDownloading(false);
    if (ok) onClose();
  }

  return (
    <AlertDialog open onOpenChange={(open) => !open && !downloading && onClose()}>
      <AlertDialogContent className="max-h-[95vh] max-w-7xl overflow-y-auto bg-white text-black dark:bg-white dark:text-black">
        <AlertDialogHeader>
          <AlertDialogTitle>
            Excel preview: {single ? target.label : `${target.month} ${target.year}`}
          </AlertDialogTitle>
          <AlertDialogDescription className="text-slate-700">
            This is what the file will contain. Download builds it again from the current data.
          </AlertDialogDescription>
        </AlertDialogHeader>

        {isPending ? (
          <TableSkeleton columns={3} rows={4} />
        ) : error ? (
          <ErrorRetry
            message={error instanceof ApiError ? retryText(error) : "Could not load the preview. Try again."}
            onRetry={() => refetch()}
          />
        ) : (
          <>
            {(!single || data.sheets.length > 1) && (
              <div className="flex flex-wrap items-end gap-4">
                <div className="flex flex-col gap-2">
                  <Label htmlFor="preview-sheet">Sheet</Label>
                  <NativeSelect
                    id="preview-sheet"
                    value={sheet}
                    onChange={(e) => setSheet(Number(e.target.value))}
                    className="border-slate-300 bg-white text-slate-900 [&_option]:bg-white [&_option]:text-slate-900"
                  >
                    {data.sheets.map((s, i) => (
                      <option key={s.name} value={i}>
                        {s.name}
                      </option>
                    ))}
                  </NativeSelect>
                </div>
                {!single && <p className="pb-1.5 text-sm text-slate-700">{data.count} approved</p>}
              </div>
            )}
            {shown && (
              !(single ? shown.name === SUMMARY_SHEET : TABLE_SHEETS.has(shown.name))
                ? <ExcelReportSheet rows={shown.rows} name={shown.name} status={previewStatus} />
                : <ExcelDashboardSheet sheet={shown} />
            )}
          </>
        )}

        <AlertDialogFooter>
          <AlertDialogCancel
            disabled={downloading}
            className="min-w-24 border-slate-700 bg-white px-4 text-slate-900 hover:bg-slate-100 hover:text-slate-950"
          >
            Cancel
          </AlertDialogCancel>
          {/* A plain Button, not AlertDialogAction: that one closes the dialog on click, and a failed download must keep it open. */}
          <Button
            type="button"
            className="min-w-28 bg-slate-950 px-4 text-white hover:bg-slate-800"
            disabled={!data || downloading}
            aria-busy={downloading}
            onClick={download}
          >
            {downloading ? <><Loader2 aria-hidden="true" className="motion-safe:animate-spin" />Downloading…</> : "Download"}
          </Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}

function ExcelReportSheet({ rows, name, status }: { rows: string[][]; name: string; status?: string }) {
  const cell = (row: number, column: number) => rows[row]?.[column] ?? "";
  const activities = rows.slice(9, 28).filter((r) => r[0]);
  const statistics = rows.slice(30, 36).filter((r) => r[0]);

  return (
    <div role="region" aria-label={`Sheet ${name}`} tabIndex={0} className="mx-auto max-h-[70vh] w-full max-w-[1100px] overflow-auto bg-white font-sans text-[11px] text-black outline-none focus-visible:ring-3 focus-visible:ring-ring/50">
      <div className="border-b border-black pb-3 text-center">
        {/* Static public asset; Next image optimization is unnecessary for this fixed report logo. */}
        {/* eslint-disable-next-line @next/next/no-img-element */}
        <img src="/amgc-logo.png" alt="AMGC logo" className="mx-auto mb-2 h-16 w-auto object-contain" />
        <h1 className="text-lg font-bold">{cell(0, 0)}</h1>
        <h2 className="text-sm font-bold">{cell(2, 0)}</h2>
        <p className="mt-1 text-xs">{cell(3, 0)} {cell(3, 1)}</p>
      </div>

      <section className="grid grid-cols-1 border-x border-b border-black sm:grid-cols-2">
        {[[4, "Worker:"], [5, "Area of Assignment:"], [6, "Name of Church/Outreach:"], [null, "Status:"]].map(([row, label]) => (
          <div key={label} className="grid grid-cols-[minmax(10rem,auto)_1fr] border-b border-black px-2 py-1 last:border-b-0 sm:odd:border-r">
            <span className="font-bold">{label}</span>
            <span>{row === null ? (status ? <ExcelStatusBadge status={status} /> : "—") : cell(Number(row), 1) || "—"}</span>
          </div>
        ))}
      </section>

      <section className="mt-4">
        <ExcelMetricTable title="Activities" rows={activities} name={name} />
        <ExcelMetricTable title="Other Statistics" rows={statistics} name={name} names={cell(36, 1)} />
      </section>

      <section className="mt-4 border border-black">
        <h3 className="border-b border-black bg-black px-2 py-1 font-bold text-white">Report Details</h3>
        <dl className="grid gap-0">
          {[[39, "Narrative Report:"], [40, "Challenges/Problem Encountered:"], [41, "Prayer Request:"]].map(([row, label], index) => (
            <div key={label} className={`grid grid-cols-[minmax(12rem,18%)_1fr] border-b border-black last:border-b-0 ${index % 2 ? "bg-slate-50" : "bg-white"}`}>
              <dt className="border-r border-black px-2 py-1 font-bold">{label}</dt>
              <dd className="whitespace-pre-wrap px-2 py-1 [overflow-wrap:anywhere]">{cell(Number(row), 1) || "—"}</dd>
            </div>
          ))}
        </dl>
      </section>
    </div>
  );
}

// names: the sheet puts Names (row 37) right after the last statistic; shown as a label row, then the names below it.
function ExcelMetricTable({ title, rows, name, names }: { title: string; rows: string[][]; name: string; names?: string }) {
  const zebra = (index: number) => (index % 2 ? "bg-slate-50 hover:bg-slate-100" : "bg-white hover:bg-slate-100");
  return (
    <div className="mb-3 overflow-x-auto border border-black last:mb-0">
      <div className="bg-black px-2 py-1 font-bold text-white">{title}</div>
      <Table aria-label={`Sheet ${name} ${title}`} className="table-fixed border-collapse text-[11px]">
        <TableHeader>
          <TableRow className="bg-slate-200 hover:bg-slate-200">
            <TableHead className="w-[30%] border-r border-black px-2 py-1 font-bold text-black h-10 normal-case tracking-normal text-[11px]">Activities</TableHead>
            {Array.from({ length: 5 }, (_, i) => <TableHead key={i} className="border-r border-black px-2 py-1 text-center font-bold text-black h-10 normal-case tracking-normal text-[11px]">Week {i + 1}</TableHead>)}
            <TableHead className="px-2 py-1 text-center font-bold text-black h-10 normal-case tracking-normal text-[11px]">Average</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map((cells, index) => (
            <TableRow key={index} className={zebra(index)}>
              {cells.slice(0, 7).map((text, column) => (
                <TableCell key={column} className={`${column < 6 ? "border-r border-black" : ""} px-2 py-1 text-black ${column === 0 ? "font-medium" : "text-center"}`}>
                  {text || "—"}
                </TableCell>
              ))}
            </TableRow>
          ))}
          {names !== undefined && (
            <>
              <TableRow className={zebra(rows.length)}>
                <TableCell className="border-r border-black px-2 py-1 font-bold text-black">Names:</TableCell>
                {Array.from({ length: 5 }, (_, i) => <TableCell key={i} className="border-r border-black px-2 py-1" />)}
                <TableCell className="px-2 py-1" />
              </TableRow>
              <TableRow className={zebra(rows.length + 1)}>
                <TableCell colSpan={7} className="whitespace-pre-wrap px-2 py-1 text-black [overflow-wrap:anywhere]">{names || "—"}</TableCell>
              </TableRow>
            </>
          )}
        </TableBody>
      </Table>
    </div>
  );
}

// The table sheets: the bulk export's two and the single export's Summary. The backend reserves
// each name in its export, so no church sheet can take it.
const TABLE_SHEETS = new Set(["Report Dashboard", "Approved Reports"]);
const SUMMARY_SHEET = "Summary";

function ExcelDashboardSheet({ sheet }: { sheet: { name: string; rows: string[][] } }) {
  const title = sheet.rows[0]?.[0] || sheet.name;
  const headers = sheet.rows[1] ?? [];
  const rows = sheet.rows.slice(2).filter((row) => row.some(Boolean)).map((row) => row.slice(0, headers.length));

  return (
    <div role="region" aria-label={`Sheet ${sheet.name}`} tabIndex={0} className="max-h-[60vh] overflow-auto rounded-lg border border-slate-300 bg-white outline-none focus-visible:ring-3 focus-visible:ring-ring/50">
      <Table aria-label={`Sheet ${sheet.name}`} style={{ minWidth: Math.max(760, headers.length * 110) }} className="table-fixed border-collapse bg-white text-[11px] text-black">
        <caption className="sr-only">Sheet {sheet.name}</caption>
        <TableHeader>
          <TableRow className="bg-slate-100 hover:bg-slate-100">
            <TableHead colSpan={headers.length || 1} className="border-b border-slate-300 px-3 py-2 text-center h-10 text-base font-bold normal-case tracking-normal text-slate-900">
              {title}
            </TableHead>
          </TableRow>
          <TableRow className="bg-slate-200 hover:bg-slate-200">
            {headers.map((text, c) => (
              <TableHead key={c} scope="col" className={`${c < headers.length - 1 ? "border-r border-slate-300" : ""} px-3 py-2 font-bold text-slate-900 h-10 normal-case tracking-normal text-[11px] ${c === 0 ? "text-left" : "text-center"}`}>
                {text}
              </TableHead>
            ))}
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map((cells, r) => (
            <TableRow key={r} className={r % 2 ? "bg-slate-50 hover:bg-slate-100" : "bg-white hover:bg-slate-100"}>
              {cells.map((text, c) => (
                <TableCell key={c} className={`${c < headers.length - 1 ? "border-r border-slate-300" : ""} whitespace-pre-wrap px-3 py-2 text-black ${c === 0 ? "text-left" : "text-center"}`}>
                  {headers[c] === "Status" ? <ExcelStatusBadge status={text} /> : text || "—"}
                </TableCell>
              ))}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
