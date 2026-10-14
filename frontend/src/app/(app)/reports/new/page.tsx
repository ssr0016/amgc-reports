"use client";

import { useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { ReportForm } from "@/components/report-form";
import { emptyForm } from "@/lib/report-form";
import { createReport } from "@/lib/reports";
import { useSession } from "@/lib/session";

export default function NewReportPage() {
  const router = useRouter();
  const queryClient = useQueryClient();
  const { can } = useSession();
  const [initial] = useState(emptyForm);

  return (
    <main className="mx-auto flex w-full max-w-5xl flex-1 flex-col gap-6 p-4">
      <h1 className="text-2xl font-semibold tracking-tight">New report</h1>
      {can("reports:write") ? (
        <ReportForm
          initial={initial}
          submitLabel="Save report"
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
