import { Badge } from "@/components/ui/badge";

const STATUS = {
  approved: ["Approved", "success"],
  pending: ["Pending", "warning"],
  needs_revision: ["Needs Revision", "danger"],
  no_report_submitted: ["No Report Submitted", "neutral"],
} as const;

export function ReportStatusBadge({ status }: { status: string }) {
  const key = status.toLowerCase().replaceAll(" ", "_");
  const [label, variant] = STATUS[key as keyof typeof STATUS] ?? [status, "neutral"];

  return <Badge data-status={key} variant={variant}>{label}</Badge>;
}
