"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useSession } from "@/lib/session";
import { AdminReports } from "./admin-reports";
import { AuditLogs } from "./audit-logs";
import { StatsCards } from "./stats-cards";
import { Tracker } from "./tracker";
import { UsersPanel } from "./users/users-list";

// The tab lives in ?tab= (old /admin/users links land on ?tab=users). A change replaces the entry instead of
// pushing one, so Back leaves /admin rather than stepping through tabs. Keep the tab order.
export function AdminPanel() {
  const { isAdmin } = useSession();
  const router = useRouter();
  const param = useSearchParams().get("tab");
  const tab = param === "tracker" || param === "users" || param === "logs" ? param : "reports";

  return (
    <main id="main-content" tabIndex={-1} className="mx-auto flex w-full max-w-5xl flex-1 flex-col gap-6 p-4 focus:outline-none">
      <h1 className="text-2xl font-semibold tracking-tight">Admin</h1>
      {isAdmin ? (
        <>
          <StatsCards />
          <Tabs value={tab} onValueChange={(v) => router.replace(`/admin?tab=${v}`)}>
            <TabsList>
              <TabsTrigger value="reports">Reports</TabsTrigger>
              <TabsTrigger value="tracker">Tracker</TabsTrigger>
              <TabsTrigger value="users">Users</TabsTrigger>
              <TabsTrigger value="logs">Logs</TabsTrigger>
            </TabsList>
            <TabsContent value="reports">
              <AdminReports />
            </TabsContent>
            <TabsContent value="tracker">
              <Tracker />
            </TabsContent>
            <TabsContent value="users">
              <UsersPanel />
            </TabsContent>
            <TabsContent value="logs">
              <AuditLogs />
            </TabsContent>
          </Tabs>
        </>
      ) : (
        <p className="text-sm text-muted-foreground">You are not allowed to view the admin panel.</p>
      )}
    </main>
  );
}
