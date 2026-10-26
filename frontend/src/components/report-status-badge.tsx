import { Badge } from "@/components/ui/badge";

const STATUS = {
  approved: ["Approved", "border-transparent bg-emerald-100 text-emerald-800"],
  pending: ["Pending", "border-transparent bg-amber-100 text-amber-900"],
  needs_revision: ["Needs Revision", "border-transparent bg-red-100 text-red-800"],
  no_report_submitted: ["No Report Submitted", "border-transparent bg-slate-100 text-slate-700"],
} as const;

export function ReportStatusBadge({ status }: { status: string }) {
  const key = status.toLowerCase().replaceAll(" ", "_");
  const [label, className] = STATUS[key as keyof typeof STATUS] ?? [status, "border-transparent bg-slate-100 text-slate-700"];

  return <Badge data-status={key} className={className}>{label}</Badge>;
}
