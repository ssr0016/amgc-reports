"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";
import { ReportForm } from "@/components/report-form";
import { emptyForm } from "@/lib/report-form";
import { createReport, defaultPeriod, getCurrentReport } from "@/lib/reports";
import { useSession } from "@/lib/session";
import { refreshAdminViews } from "@/lib/query";

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
  const allowed = user?.role === "user" && can("reports:write");
  const existing = useQuery({
    queryKey: ["current-report", user?.user_id],
    queryFn: getCurrentReport,
    enabled: allowed,
    staleTime: 30_000,
  });
  const report = existing.data;
  useEffect(() => {
    if (!report) return;
    router.replace(report.status === "draft" || report.status === "needs_revision" ? `/reports/${report.id}/edit` : `/reports/${report.id}`);
  }, [report, router]);

  const autoSave = useCallback(async (input: Parameters<typeof createReport>[0]) => {
    const saved = await createReport(input, true);
    router.replace(`/reports/${saved.id}/edit`);
    return saved;
  }, [router]);

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
        <p role="status" className="text-sm text-muted-foreground">Opening your report…</p>
      ) : (
        <ReportForm
          initial={initial}
          draft
          storageKey={`report:${user?.user_id}:${period.month}:${period.year}`}
          submitLabel="Submit Report"
          onAutoSave={autoSave}
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
