"use client";

import { useQuery } from "@tanstack/react-query";
import {
  Church,
  CircleDot,
  FileText,
  FileWarning,
  Shield,
  ShieldAlert,
  ShieldCheck,
  UserCheck,
  UserCog,
  UserMinus,
  UserRound,
  Users,
  UserX,
  type LucideIcon,
} from "lucide-react";
import { StatCard, type StatTone } from "@/components/stat-card";
import { getStats } from "@/lib/stats";
import { useSession } from "@/lib/session";
import { KpiSkeleton } from "@/components/loading-skeletons";
import { ErrorRetry } from "@/components/error-retry";
import { statusLabel } from "@/lib/user-status";

type Card = { label: string; value: number; icon: LucideIcon; tone: StatTone; note?: string };

// The built-in roles and the four statuses get their own look; a custom role or an unknown status a plain one.
const ROLE_LOOK: Record<string, Pick<Card, "icon" | "tone">> = {
  admin: { icon: ShieldCheck, tone: "sky" },
  editor: { icon: UserCog, tone: "green" },
  user: { icon: UserRound, tone: "neutral" },
};
const STATUS_LOOK: Record<string, Pick<Card, "icon" | "tone">> = {
  active: { icon: UserCheck, tone: "green" },
  inactive: { icon: UserMinus, tone: "neutral" },
  disciplinary: { icon: ShieldAlert, tone: "amber" },
  deceased: { icon: UserX, tone: "red" },
};

// Fills the row with as many cards as fit, so any number of roles or statuses lays out evenly.
const GRID = "grid grid-cols-[repeat(auto-fill,minmax(min(100%,13rem),1fr))] gap-4 lg:gap-5";

const cardList = (cards: Card[]) => cards.map((c) => <StatCard key={c.label} {...c} />);

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
    const roles: Card[] = data.role_counts.map((r) => ({ label: r.name, value: r.count, ...(ROLE_LOOK[r.name] ?? { icon: Shield, tone: "neutral" }) }));
    const statuses: Card[] = data.status_counts.map((s) => ({
      label: statusLabel(s.status),
      value: s.count,
      ...(STATUS_LOOK[s.status] ?? { icon: CircleDot, tone: "neutral" }),
    }));
    return (
      <section aria-label="Statistics" className="flex flex-col gap-6">
        <div className="flex flex-col gap-3">
          <h2 className="text-xs font-medium tracking-wide text-muted-foreground uppercase">By role</h2>
          <div className={GRID}>{cardList([{ label: "Total Users", value: data.users_total, icon: Users, tone: "neutral" }, ...roles])}</div>
        </div>
        <div className="flex flex-col gap-3">
          <h2 className="text-xs font-medium tracking-wide text-muted-foreground uppercase">By status</h2>
          <div className={GRID}>{cardList(statuses)}</div>
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
