"use client";

import { useQuery } from "@tanstack/react-query";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { getStats } from "@/lib/stats";
import { useSession } from "@/lib/session";

// The numbers are text, so nothing here relies on colour. Counts only: the endpoint sends no report text.
export function StatsCards() {
  const { isAdmin } = useSession();
  const { data, error, isPending } = useQuery({
    queryKey: ["admin-stats"],
    queryFn: getStats,
    enabled: isAdmin,
    staleTime: 30_000,
  });

  if (!isAdmin) return null;
  if (error) {
    return (
      <p role="alert" className="text-sm text-destructive">
        {error.message}
      </p>
    );
  }
  if (isPending) {
    return <p role="status" className="text-sm text-muted-foreground">Loading…</p>;
  }

  const cards = [
    { label: "Reports", value: data.reports_total, note: `${data.month} ${data.year}` },
    { label: "Approved", value: data.reports_approved },
    { label: "Pending", value: data.reports_pending },
    { label: "Users", value: data.users_total, note: `${data.admins_total} ${data.admins_total === 1 ? "admin" : "admins"}` },
  ];
  return (
    <section aria-label="Statistics" className="grid grid-cols-2 gap-4 lg:grid-cols-4">
      {cards.map((c) => (
        <Card key={c.label} size="sm">
          <CardHeader>
            <CardDescription>{c.label}</CardDescription>
            <CardTitle className="text-2xl">{c.value}</CardTitle>
          </CardHeader>
          {c.note && <CardContent className="text-muted-foreground">{c.note}</CardContent>}
        </Card>
      ))}
    </section>
  );
}
