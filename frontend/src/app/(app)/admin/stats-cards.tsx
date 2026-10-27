"use client";

import { useQuery } from "@tanstack/react-query";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { getStats } from "@/lib/stats";
import { useSession } from "@/lib/session";
import { KpiSkeleton } from "@/components/loading-skeletons";
import { ErrorRetry } from "@/components/error-retry";

// The numbers are text, so nothing here relies on colour. Counts only: the endpoint sends no report text.
export function StatsCards() {
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

  const cards = [
    { label: "Total Users", value: data.users_total },
    { label: "Expected Reports", value: data.expected_reports, note: `${data.month} ${data.year}` },
    { label: "No Report", value: data.no_report },
    { label: "Total Churches", value: data.churches_total },
  ];
  return (
    <section aria-label="Statistics" className="grid grid-cols-2 gap-4 lg:grid-cols-4">
      {cards.map((c) => (
        <Card key={c.label} size="sm">
          <CardHeader>
            <CardDescription>{c.label}</CardDescription>
          <CardTitle className="font-mono text-2xl tabular-nums">{c.value}</CardTitle>
          </CardHeader>
          {c.note && <CardContent className="text-muted-foreground">{c.note}</CardContent>}
        </Card>
      ))}
    </section>
  );
}
