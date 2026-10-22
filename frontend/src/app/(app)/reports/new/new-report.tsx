"use client";

import { useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { ReportForm } from "@/components/report-form";
import { emptyForm } from "@/lib/report-form";
import { createReport, defaultPeriod } from "@/lib/reports";
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

  return (
    <main id="main-content" tabIndex={-1} className="mx-auto flex w-full max-w-5xl flex-1 flex-col gap-6 p-4 focus:outline-none">
      <h1 className="text-2xl font-semibold tracking-tight">New report</h1>
      {can("reports:write") ? (
        <ReportForm
          initial={initial}
          submitLabel="Submit Report"
          lockedPeriod={period}
          onSubmit={async (input) => {
            await createReport(input);
            await Promise.all([queryClient.invalidateQueries({ queryKey: ["reports"] }), refreshAdminViews(queryClient)]);
            router.replace("/reports");
          }}
          onCancel={() => router.push("/reports")}
        />
      ) : (
        <p className="text-sm text-muted-foreground">You are not allowed to create reports.</p>
      )}
    </main>
  );
}
