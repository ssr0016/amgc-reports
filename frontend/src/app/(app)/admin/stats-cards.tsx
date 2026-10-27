"use client";

import { useQuery } from "@tanstack/react-query";
import { Church, FileText, FileWarning, ShieldCheck, UserCheck, Users, UserX, type LucideIcon } from "lucide-react";
import { StatCard, type StatTone } from "@/components/stat-card";
import { getStats } from "@/lib/stats";
import { useSession } from "@/lib/session";
import { KpiSkeleton } from "@/components/loading-skeletons";
import { ErrorRetry } from "@/components/error-retry";

// The numbers are text, so nothing here relies on colour. Counts only: the endpoint sends no report text.
export function StatsCards({ users = false }: { users?: boolean }) {
  const { can } = useSession();
  const allowed = can("tracker:read");
  const { data, error, isPending, refetch } = useQuery({
    queryKey: ["admin-stats"],
    queryFn: getStats,
    enabled: allowed,
    staleTime: 30_000,
  });

  if (!allowed) return null;
  if (error) {
    return <ErrorRetry message={error.message} onRetry={() => refetch()} />;
  }
  if (isPending) {
    return <KpiSkeleton />;
  }

  const cards: { label: string; value: number; icon: LucideIcon; tone: StatTone; note?: string }[] = users
    ? [
        { label: "Total Users", value: data.users_total, icon: Users, tone: "neutral" },
        { label: "Active Users", value: data.users_active, icon: UserCheck, tone: "green" },
        { label: "Admins", value: data.admins_total, icon: ShieldCheck, tone: "sky" },
        // Inactive, disciplinary and deceased together.
        { label: "Not Active", value: data.users_total - data.users_active, icon: UserX, tone: "amber" },
      ]
    : [
        { label: "Total Users", value: data.users_total, icon: Users, tone: "neutral" },
        { label: "Expected Reports", value: data.expected_reports, icon: FileText, tone: "green", note: `${data.month} ${data.year}` },
        { label: "No Report", value: data.no_report, icon: FileWarning, tone: "amber" },
        { label: "Total Churches", value: data.churches_total, icon: Church, tone: "sky" },
      ];
  return (
    <section aria-label="Statistics" className="grid grid-cols-2 gap-4 lg:grid-cols-4 lg:gap-5">
      {cards.map((c) => (
        <StatCard key={c.label} {...c} />
      ))}
    </section>
  );
}
