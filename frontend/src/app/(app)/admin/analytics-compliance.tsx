"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import {
  COMPLIANCE_LABELS,
  monthLabel,
  setChurchReporting,
  TIMELINESS_LABELS,
  type AttendanceAnalytics,
  type ChurchScope,
  type ComplianceCell,
} from "@/lib/analytics";
import { errorText } from "@/lib/auth-form";
import { cn } from "@/lib/utils";

// Tones follow the workbook: green approved, amber still owed or in review, red not filed, grey nothing expected.
const TONES = {
  good: "bg-emerald-100 text-emerald-900 dark:bg-emerald-950 dark:text-emerald-100",
  warn: "bg-amber-100 text-amber-900 dark:bg-amber-950 dark:text-amber-100",
  bad: "bg-red-100 text-red-900 dark:bg-red-950 dark:text-red-100",
  muted: "bg-muted text-muted-foreground",
};

export function cellText(cell: ComplianceCell): { text: string; tone: keyof typeof TONES } {
  if (cell.state === "submitted") {
    const review = COMPLIANCE_LABELS[cell.review_status ?? "pending"];
    const when = TIMELINESS_LABELS[cell.timeliness ?? ""] ?? "";
    return { text: when ? `${review} · ${when}` : review, tone: cell.review_status === "approved" ? "good" : "warn" };
  }
  const tone = cell.state === "not_filed" ? "bad" : cell.state === "awaiting_submission" || cell.state === "overdue_grace" ? "warn" : "muted";
  return { text: COMPLIANCE_LABELS[cell.state], tone };
}

const LEGEND: { text: string; tone: keyof typeof TONES; note: string }[] = [
  { text: "Approved", tone: "good", note: "submitted and approved" },
  { text: "Pending / Needs Revision", tone: "warn", note: "submitted, still in review" },
  { text: "Awaiting Submission", tone: "warn", note: "1st–15th of the next month" },
  { text: "Overdue — Grace Period", tone: "warn", note: "16th to the next month's end; still accepted, marked late" },
  { text: "Not Filed", tone: "bad", note: "window closed with no report" },
  { text: "Not Required", tone: "muted", note: "before the church's start month or after its end month" },
  { text: "Church Inactive", tone: "muted", note: "no active church account and no end month; nothing is missing" },
  { text: "Not Yet Open", tone: "muted", note: "the month has not ended" },
];

export function ComplianceMatrix({
  data,
  canEdit,
  onOpenChurch,
}: {
  data: AttendanceAnalytics;
  canEdit: boolean;
  onOpenChurch: (id: number) => void;
}) {
  const [editing, setEditing] = useState<number | null>(null);
  const churches = new Map(data.churches.map((c) => [c.id, c]));
  const t = data.compliance_totals;
  return (
    <section aria-labelledby="compliance-heading" className="flex flex-col gap-3">
      <div>
        <h2 id="compliance-heading" className="text-base font-medium">Submission Compliance</h2>
        <p className="text-sm text-muted-foreground">
          {data.default_start_month
            ? `Every church reports from ${monthLabel(data.default_start_month)} (the organization start) unless it has its own months below. `
            : "No organization start month is configured, so only months with a report are counted. "}
          Every church by activity month, whatever the status filter. {t.submitted} of {t.expected} expected reports submitted ({t.on_time} on time, {t.late} late) · {t.approved} approved · {t.pending + t.needs_revision} in review · {t.awaiting} awaiting · {t.overdue} overdue · {t.not_filed} not filed{t.outside > 0 ? ` · ${t.outside} more for months not required` : ""}.
          {t.churches_needing_review > 0 && ` ${t.churches_needing_review} ${t.churches_needing_review === 1 ? "church has" : "churches have"} no start month, so ${t.churches_needing_review === 1 ? "it is" : "they are"} not counted as expected.`}
        </p>
      </div>
      <ul aria-label="Compliance legend" className="flex flex-wrap gap-2 text-xs">
        {[...LEGEND, ...(t.churches_needing_review > 0 ? [{ text: "Start Not Set", tone: "muted" as const, note: "no organization or church start month; nothing is missing" }] : [])].map((item) => (
          <li key={item.text} className="flex items-center gap-1.5">
            <span className={cn("rounded px-1.5 py-0.5 font-medium", TONES[item.tone])}>{item.text}</span>
            <span className="text-muted-foreground">{item.note}</span>
          </li>
        ))}
      </ul>
      <div className="overflow-x-auto rounded-lg border">
        <Table aria-label="Submission compliance by church and month" aria-describedby="compliance-note">
          <TableHeader>
            <TableRow>
              <TableHead scope="col" className="sticky left-0 z-10 min-w-28 bg-background sm:min-w-48">Church</TableHead>
              <TableHead scope="col">Reporting months</TableHead>
              {data.months.map((m) => <TableHead scope="col" key={m.month} className="min-w-28">{m.name.slice(0, 3)}</TableHead>)}
              <TableHead scope="col" className="text-right">Submitted / Expected</TableHead>
              <TableHead scope="col" className="text-right">Approved</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {data.compliance.map((row) => {
              const church = churches.get(row.church_id);
              return (
                <TableRow key={row.church_id}>
                  <TableHead scope="row" className="sticky left-0 z-10 bg-background text-sm font-medium tracking-normal whitespace-normal text-foreground normal-case sm:whitespace-nowrap">
                    <button type="button" className="text-left underline-offset-4 hover:underline focus-visible:underline" onClick={() => onOpenChurch(row.church_id)}>{row.church_name}</button>
                  </TableHead>
                  <TableCell className="whitespace-nowrap">
                    {church && editing === church.id ? (
                      <EligibilityForm church={church} defaultStart={data.default_start_month} onDone={() => setEditing(null)} />
                    ) : (
                      <span className="flex items-center gap-2">
                        <span className={cn((church?.needs_review || church?.active === false) && "text-muted-foreground")}>{eligibilityText(church)}</span>
                        {canEdit && church && church.id !== 0 && <Button type="button" size="sm" variant="ghost" onClick={() => setEditing(church.id)} aria-label={`Edit reporting months for ${church.name}`}>Edit</Button>}
                      </span>
                    )}
                  </TableCell>
                  {row.cells.map((cell) => {
                    const { text, tone } = cellText(cell);
                    return (
                      <TableCell key={cell.month}>
                        <span className={cn("inline-block rounded px-1.5 py-0.5 text-xs font-medium whitespace-nowrap", TONES[tone])} title={cell.submitted_on ? `Submitted ${cell.submitted_on}${cell.duplicates ? ` · ${cell.duplicates} other report(s) for this month not counted` : ""}` : undefined}>
                          {text}{cell.state === "submitted" && !cell.expected ? " (not required)" : ""}{cell.duplicates ? " *" : ""}
                        </span>
                      </TableCell>
                    );
                  })}
                  <TableCell className="text-right tabular-nums">{row.submitted} / {row.expected}</TableCell>
                  <TableCell className="text-right tabular-nums">{row.approved}</TableCell>
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
      </div>
      <p id="compliance-note" className="text-sm text-muted-foreground">Expected counts only the months a church owed whose report window has opened. A report for a month not required is kept and shown, but does not count toward expected or coverage. Timeliness is from when the report was submitted, in Asia/Manila time. * marks a month with more than one report; only one is counted.</p>
    </section>
  );
}

function eligibilityText(church?: ChurchScope): string {
  if (!church || church.needs_review) return "Start month not set";
  const months = `${monthLabel(church.start_month)}${church.start_is_default ? " (default)" : ""} – ${church.end_month ? monthLabel(church.end_month) : "ongoing"}`;
  return church.active ? months : `${months} · inactive`;
}

// EligibilityForm sets a church's optional overrides: activity months on the church
// record. A blank first month uses the organization default.
function EligibilityForm({ church, defaultStart, onDone }: { church: ChurchScope; defaultStart: string; onDone: () => void }) {
  const queryClient = useQueryClient();
  const [start, setStart] = useState(church.start_is_default ? "" : (church.start_month ?? ""));
  const [end, setEnd] = useState(church.end_month ?? "");
  const save = useMutation({
    mutationFn: () => setChurchReporting(church.id, { start_month: start, end_month: end }),
    onSuccess: () => {
      toast.success(`Saved reporting months for ${church.name}`);
      void queryClient.invalidateQueries({ queryKey: ["analytics-attendance"] });
      onDone();
    },
    onError: (err) => toast.error(errorText(err, "Could not save the reporting months.")),
  });
  const effectiveStart = start || defaultStart;
  const invalid = end !== "" && effectiveStart !== "" && end < effectiveStart;
  return (
    <form
      className="flex flex-wrap items-end gap-2"
      onSubmit={(event) => {
        event.preventDefault();
        if (!invalid) save.mutate();
      }}
    >
      <div className="flex flex-col gap-1">
        <Label htmlFor={`start-${church.id}`} className="text-xs">First month{defaultStart ? ` (blank: ${monthLabel(defaultStart)})` : ""}</Label>
        <Input id={`start-${church.id}`} type="month" value={start} onChange={(e) => setStart(e.target.value)} className="h-8 w-40" />
      </div>
      <div className="flex flex-col gap-1">
        <Label htmlFor={`end-${church.id}`} className="text-xs">Last month (optional)</Label>
        <Input id={`end-${church.id}`} type="month" value={end} onChange={(e) => setEnd(e.target.value)} className="h-8 w-40" aria-invalid={invalid} />
      </div>
      <Button type="submit" size="sm" disabled={invalid || save.isPending}>{save.isPending ? "Saving…" : "Save"}</Button>
      <Button type="button" size="sm" variant="ghost" onClick={onDone}>Cancel</Button>
      {invalid && <p role="alert" className="w-full text-xs text-destructive">The last month must not be before the first month.</p>}
    </form>
  );
}
