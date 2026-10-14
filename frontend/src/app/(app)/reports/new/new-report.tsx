"use client";

import { useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { ReportForm } from "@/components/report-form";
import { emptyForm } from "@/lib/report-form";
import { createReport, defaultPeriod } from "@/lib/reports";
import { useSession } from "@/lib/session";

export function NewReport() {
  const router = useRouter();
  const queryClient = useQueryClient();
  const { can, user } = useSession();
  // The profile is set by an admin; worker falls back to the user's name. All three stay editable.
  const [initial] = useState(() =>
    emptyForm({
      worker: user?.worker || user?.name,
      area_assignment: user?.area_assignment,
      church_name: user?.church_name,
    }),
  );
  const [period] = useState(() => defaultPeriod());

  return (
    <main className="mx-auto flex w-full max-w-5xl flex-1 flex-col gap-6 p-4">
      <h1 className="text-2xl font-semibold tracking-tight">New report</h1>
      {can("reports:write") ? (
        <ReportForm
          initial={initial}
          submitLabel="Save report"
          lockedPeriod={period}
          onSubmit={async (input) => {
            await createReport(input);
            await queryClient.invalidateQueries({ queryKey: ["reports"] });
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
