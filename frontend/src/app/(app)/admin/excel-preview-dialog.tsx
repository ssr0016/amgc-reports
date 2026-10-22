"use client";

import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
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
import { previewExcel } from "@/lib/reports";
import { Loading } from "@/components/loading";
import { ErrorRetry } from "@/components/error-retry";

// One report (the row's Excel button) or one month (Download Bulk).
export type PreviewTarget = { reportId: number; label: string } | { month: string; year: number };

// Column letters for a sheet's width: 7 for a report sheet, 6 for the Report Dashboard (D-A19).
const columns = (rows: string[][]) => Array.from({ length: rows[0]?.length ?? 0 }, (_, i) => String.fromCharCode(65 + i));

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
    queryFn: () => previewExcel(single ? { report_id: target.reportId } : { month: target.month, year: target.year }),
    // Every opening reads the current data.
    gcTime: 0,
    retry: false,
  });
  const [sheet, setSheet] = useState(0);
  const [downloading, setDownloading] = useState(false);
  const shown = data?.sheets[sheet];

  async function download() {
    setDownloading(true);
    const ok = await onDownload();
    setDownloading(false);
    if (ok) onClose();
  }

  return (
    <AlertDialog open onOpenChange={(open) => !open && !downloading && onClose()}>
      <AlertDialogContent className="data-[size=default]:max-w-[calc(100%-2rem)] data-[size=default]:sm:max-w-4xl">
        <AlertDialogHeader>
          <AlertDialogTitle>
            Excel preview: {single ? target.label : `${target.month} ${target.year}`}
          </AlertDialogTitle>
          <AlertDialogDescription>
            This is what the file will contain. Download builds it again from the current data.
          </AlertDialogDescription>
        </AlertDialogHeader>

        {isPending ? (
          <Loading>Loading preview…</Loading>
        ) : error ? (
          <ErrorRetry
            message={error instanceof ApiError ? retryText(error) : "Could not load the preview. Try again."}
            onRetry={() => refetch()}
          />
        ) : (
          <>
            {!single && (
              <div className="flex flex-wrap items-end gap-4">
                <div className="flex flex-col gap-2">
                  <Label htmlFor="preview-sheet">Sheet</Label>
                  <NativeSelect id="preview-sheet" value={sheet} onChange={(e) => setSheet(Number(e.target.value))}>
                    {data.sheets.map((s, i) => (
                      <option key={s.name} value={i}>
                        {s.name}
                      </option>
                    ))}
                  </NativeSelect>
                </div>
                <p className="pb-1.5 text-sm text-muted-foreground">{data.count} approved</p>
              </div>
            )}
            {shown && (
              // Focusable so keyboard users can scroll it.
              <div
                role="region"
                aria-label={`Sheet ${shown.name}`}
                tabIndex={0}
                className="max-h-[60vh] overflow-auto rounded-lg border outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
              >
                <Table>
                  <caption className="sr-only">Sheet {shown.name}</caption>
                  <TableHeader>
                    <TableRow>
                      <TableHead>
                        <span className="sr-only">Row</span>
                      </TableHead>
                      {columns(shown.rows).map((c) => (
                        <TableHead key={c} scope="col">
                          {c}
                        </TableHead>
                      ))}
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {shown.rows.map((cells, r) => (
                      <TableRow key={r}>
                        <TableHead scope="row" className="text-muted-foreground">
                          {r + 1}
                        </TableHead>
                        {cells.map((text, c) => (
                          // Text nodes only: report text can hold anything.
                          <TableCell key={c} className="whitespace-pre-wrap">
                            {text}
                          </TableCell>
                        ))}
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            )}
          </>
        )}

        <AlertDialogFooter>
          <AlertDialogCancel disabled={downloading}>Cancel</AlertDialogCancel>
          {/* A plain Button, not AlertDialogAction: that one closes the dialog on click, and a failed download must keep it open. */}
          <Button type="button" disabled={!data || downloading} aria-busy={downloading} onClick={download}>
            {downloading ? "Downloading…" : "Download"}
          </Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
