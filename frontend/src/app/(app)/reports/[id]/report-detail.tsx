"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useState } from "react";
import { Can } from "@/components/can";
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { ApiError } from "@/lib/api";
import { METRIC_SECTIONS, WEEKS } from "@/lib/report-form";
import { deleteReport, getReport, type Report } from "@/lib/reports";
import { useSession } from "@/lib/session";

const textFields = [
  ["names", "Names of New Believers"],
  ["narrative_report", "Narrative Report"],
  ["challenges", "Challenges"],
  ["prayer_request", "Prayer Request"],
] as const;

const number = new Intl.NumberFormat("en-US", { maximumFractionDigits: 2 });

function Field({ label, value }: { label: string; value?: string }) {
  return (
    <div className="flex flex-col gap-1">
      <dt className="text-sm text-muted-foreground">{label}</dt>
      <dd className="whitespace-pre-wrap">{value || "—"}</dd>
    </div>
  );
}

// Only metrics with at least one reported week are listed. A reported 0 shows
// as 0, a week left blank as "—".
function Metrics({ report }: { report: Report }) {
  const sections = METRIC_SECTIONS.map((s) => ({
    title: s.title,
    metrics: s.metrics.filter((m) => WEEKS.some((w) => report.metrics[m.key]?.[w] !== undefined)),
  })).filter((s) => s.metrics.length > 0);

  if (sections.length === 0) return <p className="text-sm text-muted-foreground">No metrics were reported.</p>;
  return (
    <>
      {sections.map((s) => (
        <section key={s.title} className="flex flex-col gap-2">
          <h2 className="text-lg font-medium">{s.title}</h2>
          <Table className="min-w-[40rem] table-fixed">
            <TableHeader>
              <TableRow>
                <TableHead className="w-56">Metric</TableHead>
                {WEEKS.map((w) => (
                  <TableHead key={w}>Week {w.slice(4)}</TableHead>
                ))}
              </TableRow>
            </TableHeader>
            <TableBody>
              {s.metrics.map((m) => (
                <TableRow key={m.key}>
                  <TableCell className="font-medium">{m.label}</TableCell>
                  {WEEKS.map((w) => {
                    const value = report.metrics[m.key]?.[w];
                    return <TableCell key={w}>{value === undefined ? "—" : number.format(value)}</TableCell>;
                  })}
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </section>
      ))}
      <p className="text-sm text-muted-foreground">Metrics that are not listed were not reported.</p>
    </>
  );
}

export function ReportDetail() {
  const router = useRouter();
  const queryClient = useQueryClient();
  const { can } = useSession();
  const reportId = Number(useParams<{ id: string }>().id);
  const valid = Number.isInteger(reportId) && reportId > 0;
  const allowed = can("reports:read");
  const [confirming, setConfirming] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState("");
  const [gone, setGone] = useState(false);

  // gcTime 0: always the current copy, never one from before an edit.
  const { data, error, isPending } = useQuery({
    queryKey: ["report", reportId],
    queryFn: () => getReport(reportId),
    enabled: valid && allowed,
    gcTime: 0,
  });

  async function confirmDelete() {
    setDeleting(true);
    setDeleteError("");
    try {
      await deleteReport(reportId);
      queryClient.removeQueries({ queryKey: ["report", reportId] });
      await queryClient.invalidateQueries({ queryKey: ["reports"] });
      router.replace("/reports");
    } catch (err) {
      setDeleting(false);
      if (err instanceof ApiError && err.status === 404) {
        // Already deleted elsewhere: nothing left to confirm.
        setConfirming(false);
        setGone(true);
      } else if (err instanceof ApiError) {
        setDeleteError(err.message + (err.retryAfter ? ` Try again in ${err.retryAfter}s.` : ""));
      } else {
        setDeleteError("Could not delete the report. Try again.");
      }
    }
  }

  let body: React.ReactNode;
  if (!allowed) {
    body = <p className="text-sm text-muted-foreground">You are not allowed to view reports.</p>;
  } else if (!valid || gone || (error instanceof ApiError && error.status === 404)) {
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
    const period = `${data.month} ${data.year}`;
    body = (
      <>
        <div className="flex flex-wrap items-center justify-between gap-4">
          <div className="flex items-center gap-3">
            <h1 className="text-2xl font-semibold tracking-tight">{period}</h1>
            <Badge variant={data.completed ? "default" : "secondary"}>
              {data.completed ? "Completed" : "Pending"}
            </Badge>
            {data.completed && data.completed_at && (
              <span className="text-sm text-muted-foreground">
                Approved {new Date(data.completed_at).toLocaleDateString("en-US", { dateStyle: "medium" })}
              </span>
            )}
          </div>
          <div className="flex gap-2">
            {/* F9 adds Approve and Unapprove here */}
            <Can permission="reports:write">
              <Button asChild variant="outline">
                <Link href={`/reports/${data.id}/edit`}>Edit</Link>
              </Button>
            </Can>
            <Can permission="reports:delete">
              <Button variant="destructive" onClick={() => setConfirming(true)}>
                Delete
              </Button>
            </Can>
          </div>
        </div>

        <section className="flex flex-col gap-4">
          <h2 className="text-lg font-medium">Basic Information</h2>
          <dl className="grid gap-4 sm:grid-cols-3">
            <Field label="Worker Name" value={data.worker} />
            <Field label="Area Assignment" value={data.area_assignment} />
            <Field label="Church Name" value={data.church_name} />
          </dl>
        </section>

        <Metrics report={data} />

        <section className="flex flex-col gap-4 border-t pt-6">
          <h2 className="text-lg font-medium">Report Details</h2>
          <dl className="flex flex-col gap-4">
            {textFields.map(([name, label]) => (
              <Field key={name} label={label} value={data[name]} />
            ))}
          </dl>
        </section>

        <AlertDialog open={confirming} onOpenChange={(open) => !deleting && setConfirming(open)}>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>Delete {period}?</AlertDialogTitle>
              <AlertDialogDescription>
                You can file this month again afterwards.
              </AlertDialogDescription>
            </AlertDialogHeader>
            {deleteError && (
              <p role="alert" className="text-sm text-destructive">
                {deleteError}
              </p>
            )}
            <AlertDialogFooter>
              <AlertDialogCancel disabled={deleting}>Cancel</AlertDialogCancel>
              {/* A plain Button, not AlertDialogAction: that one closes the dialog on click, and a failed delete must keep it open. */}
              <Button variant="destructive" onClick={confirmDelete} disabled={deleting}>
                {deleting ? "Deleting…" : "Delete"}
              </Button>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </>
    );
  }

  return (
    <main className="mx-auto flex w-full max-w-5xl flex-1 flex-col gap-6 p-4">
      <Link href="/reports" className="text-sm underline underline-offset-4">
        ← Back to reports
      </Link>
      {body}
    </main>
  );
}
