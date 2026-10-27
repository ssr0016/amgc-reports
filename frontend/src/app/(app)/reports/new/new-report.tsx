"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { toast } from "sonner";
import { ReportForm } from "@/components/report-form";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { emptyForm } from "@/lib/report-form";
import { createReport, defaultPeriod, deleteReport, getCurrentReport } from "@/lib/reports";
import { useSession } from "@/lib/session";
import { refreshAdminViews } from "@/lib/query";
import { Loader2 } from "lucide-react";

export function NewReport() {
  const router = useRouter();
  const queryClient = useQueryClient();
  const { can, user } = useSession();
  // The report's worker is the user's name; area and church come from the profile an
  // admin sets. All three stay editable.
  const [initial] = useState(() =>
    emptyForm({
      worker: user?.name,
      area_assignment: user?.area_assignment,
      church_name: user?.church_name,
    }),
  );
  const [period] = useState(() => defaultPeriod());
  const [confirmOpen, setConfirmOpen] = useState(false);
  const allowed = user?.role === "user" && can("reports:write");
  const existing = useQuery({
    queryKey: ["current-report", user?.user_id],
    queryFn: getCurrentReport,
    enabled: allowed,
    staleTime: 30_000,
  });
  const report = existing.data;
  const deleteDraft = useMutation({
    mutationFn: () => deleteReport(report!.id),
    onSuccess: async () => {
      queryClient.setQueryData(["current-report", user?.user_id], null);
      setConfirmOpen(false);
      await Promise.all([queryClient.invalidateQueries({ queryKey: ["reports"] }), refreshAdminViews(queryClient)]);
    },
  });

  const decision = report && (
    <Card>
      <CardHeader>
        <CardTitle>
          {report.status === "draft"
            ? `You have a draft for ${report.month} ${report.year}.`
            : report.status === "needs_revision"
              ? `Your report for ${report.month} ${report.year} needs revision. Please check the admin notes and update it.`
              : `You already submitted a report for ${report.month} ${report.year}.`}
        </CardTitle>
      </CardHeader>
      <CardContent>
        {report.status === "draft" && <p className="text-sm text-muted-foreground">Continue your saved report or start over with a blank form.</p>}
      </CardContent>
      <CardFooter className="flex flex-wrap gap-2">
        {report.status === "draft" ? (
          <>
            <Button asChild><Link href={`/reports/${report.id}/edit`}>Continue Draft</Link></Button>
            <Button variant="outline" onClick={() => setConfirmOpen(true)}>Start New</Button>
          </>
        ) : report.status === "needs_revision" ? (
          <Button asChild><Link href={`/reports/${report.id}/edit`}>Revise Report</Link></Button>
        ) : (
          <Button asChild><Link href={`/reports/${report.id}`}>View Report</Link></Button>
        )}
      </CardFooter>
    </Card>
  );

  return (
    <main id="main-content" tabIndex={-1} className="mx-auto flex w-full max-w-5xl flex-1 flex-col gap-6 p-4 focus:outline-none">
      <h1 className="text-2xl font-medium tracking-tight">New report</h1>
      {!allowed ? (
        <p className="text-sm text-muted-foreground">You are not allowed to create reports.</p>
      ) : existing.isPending ? (
        <p role="status" className="text-sm text-muted-foreground">Checking this reporting period…</p>
      ) : existing.isError ? (
        <p role="alert" className="text-sm text-destructive">Could not check this reporting period. Please try again.</p>
      ) : report ? (
        <>
          {decision}
          <AlertDialog open={confirmOpen} onOpenChange={setConfirmOpen}>
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>Start a new report?</AlertDialogTitle>
                <AlertDialogDescription>This permanently deletes your saved draft for {report.month} {report.year}.</AlertDialogDescription>
              </AlertDialogHeader>
              {deleteDraft.isError && <p role="alert" className="text-sm text-destructive">Could not delete the draft. Please try again.</p>}
              <AlertDialogFooter>
                <AlertDialogCancel disabled={deleteDraft.isPending}>Keep Draft</AlertDialogCancel>
                <AlertDialogAction variant="destructive" disabled={deleteDraft.isPending} onClick={() => deleteDraft.mutate()}>
                  {deleteDraft.isPending ? <><Loader2 aria-hidden="true" className="motion-safe:animate-spin" />Deleting…</> : "Delete draft"}
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        </>
      ) : (
        <ReportForm
          initial={initial}
          draft
          submitLabel="Submit Report"
          onSaveDraft={async (input) => {
            const draft = await createReport(input, true);
            toast.success("Draft saved");
            router.replace(`/reports/${draft.id}/edit`);
          }}
          lockedPeriod={period}
          onSubmit={async (input) => {
            await createReport(input);
            toast.success("Report submitted");
            await Promise.all([queryClient.invalidateQueries({ queryKey: ["reports"] }), refreshAdminViews(queryClient)]);
            router.replace("/reports");
          }}
          onCancel={() => router.push("/reports")}
        />
      )}
    </main>
  );
}
