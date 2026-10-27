"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useParams, useRouter } from "next/navigation";
import { toast } from "sonner";
import { ReportForm } from "@/components/report-form";
import { ApiError } from "@/lib/api";
import { fromReport, isRestricted } from "@/lib/report-form";
import { getReport, submitReport, updateReport } from "@/lib/reports";
import { useSession } from "@/lib/session";
import { refreshAdminViews } from "@/lib/query";

export function EditReport() {
  const router = useRouter();
  const queryClient = useQueryClient();
  const { can, user } = useSession();
  const reportId = Number(useParams<{ id: string }>().id);
  const valid = Number.isInteger(reportId) && reportId > 0;
  const allowed = can("reports:read");

  // gcTime 0: PUT replaces everything, so the form must start from a fresh
  // load, never from a cached copy that may be out of date.
  const { data, error, isPending } = useQuery({
    queryKey: ["report", reportId],
    queryFn: () => getReport(reportId),
    enabled: valid && allowed,
    gcTime: 0,
  });

  // Anyone who may read a report may open it; only its owner, or someone with write_any, may change it.
  // The server enforces the same rule (404 on save), so this only keeps the form from pretending.
  const editable = !data || (can("reports:write") && (data.created_by === user?.user_id || can("reports:write_any")));

  let body: React.ReactNode;
  if (!allowed) {
    body = <p className="text-sm text-muted-foreground">You are not allowed to view reports.</p>;
  } else if (!valid || (error instanceof ApiError && error.status === 404)) {
    // The API gives 404 for missing, deleted and not-yours alike, by design.
    body = <p className="text-sm text-muted-foreground">Report not found.</p>;
  } else if (error) {
    body = (
      <p role="alert" className="text-sm text-destructive">
        {error.message}
      </p>
    );
  } else if (isPending) {
    body = <p role="status" className="text-sm text-muted-foreground">Loading…</p>;
  } else {
    body = (
      <ReportForm
        initial={fromReport(data)}
        draft={data.status === "draft"}
        submitLabel="Save Changes"
        onSaveDraft={data.status === "draft" ? async (input) => {
          await updateReport(reportId, input);
          toast.success("Draft saved");
          await queryClient.invalidateQueries({ queryKey: ["report", reportId] });
        } : undefined}
        revision={data.status === "needs_revision"}
        lockedPeriod={{ month: data.month, year: data.year }}
        readOnly={!editable}
        restricted={isRestricted(data, user?.user_id, can)}
        onSubmit={async (input) => {
          await updateReport(reportId, input);
          if (data.status === "draft") {
            await submitReport(reportId);
            toast.success("Report submitted");
          } else {
            toast.success("Report saved");
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
