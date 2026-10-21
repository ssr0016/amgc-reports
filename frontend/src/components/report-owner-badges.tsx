import { Badge } from "@/components/ui/badge";
import type { Report } from "@/lib/reports";

export function ReportOwnerBadges({ report }: { report: Pick<Report, "owner_status" | "owner_deleted_at"> }) {
  return (
    <>
      {report.owner_deleted_at && <Badge variant="secondary">Deleted user</Badge>}
      {report.owner_status === "deceased" && <Badge variant="outline">Archived</Badge>}
    </>
  );
}
