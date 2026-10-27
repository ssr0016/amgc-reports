"use client";

import { useQuery } from "@tanstack/react-query";
import {
  Church,
  CircleDot,
  FileText,
  FileWarning,
  Shield,
  Users,
  type LucideIcon,
} from "lucide-react";
import { StatCard, type StatTone } from "@/components/stat-card";
import { getStats } from "@/lib/stats";
import { useSession } from "@/lib/session";
import { KpiSkeleton } from "@/components/loading-skeletons";
import { ErrorRetry } from "@/components/error-retry";
import { statusLabel } from "@/lib/user-status";

type Card = { label: string; value: number; icon: LucideIcon; tone: StatTone; note?: string };

const cardList = (cards: Card[]) => cards.map((c) => <StatCard key={c.label} {...c} />);

// One row per non-zero entry; the label is capitalised in CSS so role names stay as stored.
const breakdown = (rows: { label: string; count: number }[]) => (
  <dl className="flex flex-col divide-y divide-white/[0.06]">
    {rows
      .filter((r) => r.count > 0)
      .map((r) => (
        <div key={r.label} className="flex items-center justify-between py-2 first:pt-0 last:pb-0">
          <dt className="capitalize">{r.label}</dt>
          <dd className="font-semibold text-foreground tabular-nums">{r.count}</dd>
        </div>
      ))}
  </dl>
);

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

  if (users) {
    return (
      <section aria-label="Statistics" className="flex flex-col gap-4 lg:gap-5">
        <StatCard label="Total Users" value={data.users_total} icon={Users} tone="neutral" />
        <div className="grid gap-4 md:grid-cols-2 lg:gap-5">
          <StatCard label="Roles Breakdown" icon={Shield} tone="sky" note={breakdown(data.role_counts.map((r) => ({ label: r.name, count: r.count })))} />
          <StatCard
            label="Statuses Breakdown"
            icon={CircleDot}
            tone="green"
            note={breakdown(data.status_counts.map((s) => ({ label: statusLabel(s.status), count: s.count })))}
          />
        </div>
      </section>
    );
  }

  const cards: Card[] = [
    { label: "Total Users", value: data.users_total, icon: Users, tone: "neutral" },
    { label: "Expected Reports", value: data.expected_reports, icon: FileText, tone: "green", note: `${data.month} ${data.year}` },
    { label: "No Report", value: data.no_report, icon: FileWarning, tone: "amber" },
    { label: "Total Churches", value: data.churches_total, icon: Church, tone: "sky" },
  ];
  return (
    <section aria-label="Statistics" className="grid grid-cols-2 gap-4 lg:grid-cols-4 lg:gap-5">
      {cardList(cards)}
    </section>
  );
}
