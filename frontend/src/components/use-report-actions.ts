"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { toast } from "sonner";
import { ApiError, saveFile } from "@/lib/api";
import { errorText } from "@/lib/auth-form";
import { exportReport, requestRevision, toggleComplete, type Report } from "@/lib/reports";
import { refreshAdminViews, usePendingVariables } from "@/lib/query";
import type { PreviewTarget } from "@/app/(app)/admin/excel-preview-dialog";

export function useReportActions() {
  // The endpoint toggles: the button says which way the row goes, and the server has the last word.
  // A row stays busy until the list refetches, because a second click before the answer would undo the first.
  const queryClient = useQueryClient();
  const approve = useMutation({
    mutationKey: ["toggle-complete"],
    mutationFn: (r: Report) => toggleComplete(r.id),
    onSuccess: (meta, r) => {
      // The server decides (another admin may have approved it first, and this click undid that), so say what it did.
      const result = meta.status === "needs_revision" ? "needs revision" : meta.status ? "approved" : meta.completed ? "approved" : "unapproved";
      toast.success(`Report ${result}: ${r.month} ${r.year}`);
      refreshAdminViews(queryClient); // the cards above and the Tracker count these reports
      return queryClient.invalidateQueries({ queryKey: ["reports"] });
    },
    onError: (err, r) => {
      if (err instanceof ApiError && err.status === 404) {
        toast.error(`${r.month} ${r.year} no longer exists.`);
        return queryClient.invalidateQueries({ queryKey: ["reports"] });
      }
      toast.error(errorText(err, "Could not update the report. Try again."));
    },
  });
  const busy = usePendingVariables<Report>(["toggle-complete"]).map((r) => r.id);
  const revision = useMutation({
    mutationKey: ["request-revision"],
    mutationFn: (r: Report) => requestRevision(r.id),
    onSuccess: (_, r) => {
      toast.success(`Revision requested: ${r.month} ${r.year}`);
      refreshAdminViews(queryClient);
      return queryClient.invalidateQueries({ queryKey: ["reports"] });
    },
    onError: (err, r) => toast.error(errorText(err, `Could not request revision for ${r.month} ${r.year}.`)),
  });
  const revisionBusy = usePendingVariables<Report>(["request-revision"]).map((r) => r.id);

  // Excel downloads, keyed "bulk" or by a row's id, so each button waits for its own file.
  const download = useMutation({
    mutationKey: ["excel-download"],
    mutationFn: ({ run }: { key: string; run: () => ReturnType<typeof exportReport> }) => run(),
    onSuccess: (file) => {
      saveFile(file);
      toast.success(`Downloaded ${file.filename}`);
    },
    onError: (err) => toast.error(errorText(err, "Could not download the file. Try again.")),
  });
  const downloading = usePendingVariables<{ key: string }>(["excel-download"]).map((d) => d.key);
  // The open Excel preview (D-A15): its target, and the download its Download button runs.
  const [preview, setPreview] = useState<{ key: string; target: PreviewTarget; run: () => ReturnType<typeof exportReport> } | null>(null);
  // The open report preview (20e): just the report id.
  const [reportPreview, setReportPreview] = useState<number | null>(null);

  return { approve, revision, busy: [...busy, ...revisionBusy], download, downloading, preview, setPreview, reportPreview, setReportPreview };
}
