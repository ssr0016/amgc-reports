"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useSession } from "@/lib/session";
import { AdminReports } from "./admin-reports";
import { AuditLogs } from "./audit-logs";
import { RolesPanel } from "./roles";
import { StatsCards } from "./stats-cards";
import { Tracker } from "./tracker";
import { UsersPanel } from "./users/users-list";

// The tab lives in ?tab= (old /admin/users links land on ?tab=users). A change replaces the entry instead of
// pushing one, so Back leaves /admin rather than stepping through tabs. Keep the tab order.
export function AdminPanel() {
  const { isAdmin, can, canAdmin } = useSession();
  const router = useRouter();
  const param = useSearchParams().get("tab");

  // Each tab shows only to those who may use it (37b); the server still enforces every route.
  const tabs = [
    { value: "reports", label: "Reports", allowed: can("reports:approve") || can("reports:export"), panel: <AdminReports /> },
    { value: "tracker", label: "Tracker", allowed: can("tracker:read"), panel: <Tracker /> },
    { value: "users", label: "Users", allowed: can("users:list"), panel: <UsersPanel /> },
    { value: "roles", label: "Roles", allowed: isAdmin, panel: <RolesPanel /> },
    { value: "logs", label: "Logs", allowed: can("audit:read"), panel: <AuditLogs /> },
  ].filter((t) => t.allowed);
  // A missing, unknown or not-allowed ?tab= lands on the first allowed tab.
  const tab = tabs.find((t) => t.value === param)?.value ?? tabs[0]?.value;

  return (
    <main id="main-content" tabIndex={-1} className="mx-auto flex w-full max-w-5xl flex-1 flex-col gap-6 p-4 focus:outline-none">
      <h1 className="text-2xl font-semibold tracking-tight">Admin</h1>
      {canAdmin && tab ? (
        <>
          <StatsCards />
          <Tabs value={tab} onValueChange={(v) => router.replace(`/admin?tab=${v}`)}>
            <TabsList>
              {tabs.map((t) => (
                <TabsTrigger key={t.value} value={t.value}>
                  {t.label}
                </TabsTrigger>
              ))}
            </TabsList>
            {tabs.map((t) => (
              <TabsContent key={t.value} value={t.value}>
                {t.panel}
              </TabsContent>
            ))}
          </Tabs>
        </>
      ) : (
        <p className="text-sm text-muted-foreground">You are not allowed to view the admin panel.</p>
      )}
    </main>
  );
}
