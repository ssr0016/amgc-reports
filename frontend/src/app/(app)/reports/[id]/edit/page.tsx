"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useParams, useRouter } from "next/navigation";
import { ReportForm } from "@/components/report-form";
import { ApiError } from "@/lib/api";
import { fromReport } from "@/lib/report-form";
import { getReport, updateReport } from "@/lib/reports";
import { useSession } from "@/lib/session";

export default function EditReportPage() {
  const router = useRouter();
  const queryClient = useQueryClient();
  const { can } = useSession();
  const reportId = Number(useParams<{ id: string }>().id);
  const valid = Number.isInteger(reportId) && reportId > 0;
  const allowed = can("reports:write");

  // gcTime 0: PUT replaces everything, so the form must start from a fresh
  // load, never from a cached copy that may be out of date.
  const { data, error, isPending } = useQuery({
    queryKey: ["report", reportId],
    queryFn: () => getReport(reportId),
    enabled: valid && allowed,
    gcTime: 0,
  });

  let body: React.ReactNode;
  if (!allowed) {
    body = <p className="text-sm text-muted-foreground">You are not allowed to edit reports.</p>;
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
    body = <p className="text-sm text-muted-foreground">Loading…</p>;
  } else {
    body = (
      <ReportForm
        initial={fromReport(data)}
        submitLabel="Save changes"
        onSubmit={async (input) => {
          await updateReport(reportId, input);
          await queryClient.invalidateQueries({ queryKey: ["reports"] });
          router.replace("/reports");
        }}
        onCancel={() => router.push("/reports")}
      />
    );
  }

  return (
    <main className="mx-auto flex w-full max-w-5xl flex-1 flex-col gap-6 p-4">
      <h1 className="text-2xl font-semibold tracking-tight">Edit report</h1>
      {body}
    </main>
  );
}
