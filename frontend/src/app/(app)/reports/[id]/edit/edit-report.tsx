"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { toast } from "sonner";
import { ReportForm } from "@/components/report-form";
import { ApiError } from "@/lib/api";
import { fromReport, isRestricted } from "@/lib/report-form";
import { getReport, submitReport, updateReport } from "@/lib/reports";
import { reportStorageKey } from "@/lib/report-draft-storage";
import { useSession } from "@/lib/session";
import { refreshAdminViews } from "@/lib/query";
import { TableSkeleton } from "@/components/loading-skeletons";

export function EditReport({ viewOnly = false, id }: { viewOnly?: boolean; id: number }) {
  const router = useRouter();
  const queryClient = useQueryClient();
  const { can, user } = useSession();
  const reportId = id;
  const valid = Number.isInteger(reportId) && reportId > 0;
  const allowed = can("reports:read");
  const [deleted, setDeleted] = useState(false);

  // gcTime 0: PUT replaces everything, so the form must start from a fresh
  // load, never from a cached copy that may be out of date.
  const { data, error, isPending } = useQuery({
    queryKey: ["report", reportId],
    queryFn: () => getReport(reportId),
    enabled: valid && allowed,
    gcTime: 0,
    refetchOnWindowFocus: true,
  });


  async function markDeleted() {
    setDeleted(true);
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ["report", reportId] }),
      queryClient.invalidateQueries({ queryKey: ["current-report", user?.user_id] }),
      queryClient.invalidateQueries({ queryKey: ["reports"] }),
    ]);
  }

  async function handleMutationError(err: unknown): Promise<never> {
    if (err instanceof ApiError && err.status === 404) await markDeleted();
    throw err;
  }

  // Anyone who may read a report may open it; only its owner, or someone with write_any, may change it.
  // The server enforces the same rule (404 on save), so this only keeps the form from pretending.
  const editable = !viewOnly && (!data || (can("reports:write") && (data.created_by === user?.user_id || can("reports:write_any"))));

  let body: React.ReactNode;
  if (!allowed) {
    body = <p className="text-sm text-muted-foreground">You are not allowed to view reports.</p>;
  } else if (!valid || deleted || (error instanceof ApiError && error.status === 404)) {
    // The API gives 404 for missing, deleted and not-yours alike, by design.
    body = (
      <div className="flex flex-col gap-3">
        <p className="text-sm text-muted-foreground">This report has been deleted</p>
        <Link className="text-sm font-medium text-primary underline-offset-4 hover:underline" href="/reports">
          Back to Community Reports
        </Link>
      </div>
    );
  } else if (error) {
    body = (
      <p role="alert" className="text-sm text-destructive">
        {error.message}
      </p>
    );
  } else if (isPending) {
    body = <TableSkeleton columns={2} rows={4} />;
  } else {
    body = (
      <ReportForm
        initial={fromReport(data)}
        initialUserNotes={data.user_notes ?? ""}
        draft={data.status === "draft"}
        submitLabel="Submit Report"
        storageKey={reportStorageKey(reportId)}
        // Drafts only: saving a Needs Revision report returns it to Pending, so an autosave would resubmit it half-done.
        onAutoSave={editable && data.status === "draft" ? async (input) => {
          try {
            await updateReport(reportId, input);
          } catch (err) {
            await handleMutationError(err);
          }
          await queryClient.invalidateQueries({ queryKey: ["report", reportId] });
        } : undefined}
        revision={data.status === "needs_revision"}
        lockedPeriod={{ month: data.month, year: data.year }}
        readOnly={!editable}
        restricted={isRestricted(data, user?.user_id, can)}
        onSubmit={async (input) => {
          try {
            await updateReport(reportId, input);
            // The PUT already moves a Needs Revision report back to Pending; submit only accepts drafts.
            if (data.status === "draft") {
              await submitReport(reportId);
              toast.success("Report submitted");
            } else if (data.status === "needs_revision") {
              toast.success("Report resubmitted");
            } else {
              toast.success("Report saved");
            }
          } catch (err) {
            await handleMutationError(err);
          }
          await Promise.all([queryClient.invalidateQueries({ queryKey: ["reports"] }), refreshAdminViews(queryClient)]);
          router.replace("/reports");
        }}
        onCancel={() => router.push("/reports")}
      />
    );
  }

  return (
    <main id="main-content" tabIndex={-1} className="mx-auto flex w-full max-w-5xl flex-1 flex-col gap-6 p-4 focus:outline-none">
      <h1 className="text-2xl font-medium tracking-tight">{editable ? "Edit report" : "View report"}</h1>
      {body}
    </main>
  );
}
