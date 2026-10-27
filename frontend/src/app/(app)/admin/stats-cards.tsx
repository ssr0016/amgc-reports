"use client";

import { useQuery } from "@tanstack/react-query";
import { Church, FileText, FileWarning, ShieldCheck, UserCheck, Users, UserX, type LucideIcon } from "lucide-react";
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { getStats } from "@/lib/stats";
import { useSession } from "@/lib/session";
import { KpiSkeleton } from "@/components/loading-skeletons";
import { ErrorRetry } from "@/components/error-retry";

// Icon tiles are decorative; the label carries the meaning.
const TONE = {
  neutral: "bg-muted text-foreground",
  green: "bg-primary/10 text-primary",
  sky: "bg-sky-500/10 text-sky-400",
  amber: "bg-amber-500/10 text-amber-400",
};

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

  const cards: { label: string; value: number; icon: LucideIcon; tone: keyof typeof TONE; note?: string }[] = users
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
        <Card
          key={c.label}
          className="border-white/[0.06] bg-card/80 bg-linear-to-b from-white/[0.03] to-transparent shadow-sm transition-shadow duration-200 hover:shadow-lg hover:shadow-black/30"
        >
          <CardHeader className="gap-2">
            <CardDescription className="text-xs font-medium tracking-wide uppercase">{c.label}</CardDescription>
            <CardTitle className="text-3xl font-semibold tracking-tight tabular-nums">{c.value}</CardTitle>
            <CardAction>
              <span aria-hidden="true" className={`grid size-10 place-items-center rounded-lg ${TONE[c.tone]}`}>
                <c.icon className="size-5" />
              </span>
            </CardAction>
          </CardHeader>
          {c.note && <CardContent className="text-muted-foreground">{c.note}</CardContent>}
        </Card>
      ))}
    </section>
  );
}
