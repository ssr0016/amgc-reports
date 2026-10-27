import { Badge } from "@/components/ui/badge";

// The Logs page tones: soft tint, dark text and a faint ring (light text in the dark theme) to stay above 4.5:1 for axe.
const slate = "bg-slate-50 text-slate-700 ring-slate-500/20 dark:bg-slate-500/10 dark:text-slate-300 dark:ring-slate-400/30";
const STATUS = {
  approved: ["Approved", "bg-green-50 text-green-800 ring-green-600/20 dark:bg-green-500/10 dark:text-green-300 dark:ring-green-400/30"],
  pending: ["Pending", "bg-amber-50 text-amber-800 ring-amber-600/20 dark:bg-amber-500/10 dark:text-amber-300 dark:ring-amber-400/30"],
  needs_revision: ["Needs Revision", "bg-red-50 text-red-800 ring-red-600/20 dark:bg-red-500/10 dark:text-red-300 dark:ring-red-400/30"],
  no_report_submitted: ["No Report Submitted", slate],
} as const;

export function ReportStatusBadge({ status }: { status: string }) {
  const key = status.toLowerCase().replaceAll(" ", "_");
  const [label, className] = STATUS[key as keyof typeof STATUS] ?? [status, slate];

  return <Badge data-status={key} className={`ring-1 ring-inset ${className}`}>{label}</Badge>;
}
