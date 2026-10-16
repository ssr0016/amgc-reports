"use client";

import { useSearchParams } from "next/navigation";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useSession } from "@/lib/session";
import { AdminReports } from "./admin-reports";
import { StatsCards } from "./stats-cards";
import { UsersPanel } from "./users/users-list";

// Tab state lives in memory; ?tab=users only picks the tab on arrival (old /admin/users links land here). Keep the tab order.
export function AdminPanel() {
  const { isAdmin } = useSession();
  const initialTab = useSearchParams().get("tab") === "users" ? "users" : "reports";

  return (
    <main id="main-content" tabIndex={-1} className="mx-auto flex w-full max-w-5xl flex-1 flex-col gap-6 p-4 focus:outline-none">
      <h1 className="text-2xl font-semibold tracking-tight">Admin</h1>
      {isAdmin ? (
        <>
          <StatsCards />
          <Tabs defaultValue={initialTab}>
            <TabsList>
              <TabsTrigger value="reports">Reports</TabsTrigger>
              <TabsTrigger value="users">Users</TabsTrigger>
            </TabsList>
            <TabsContent value="reports">
              <AdminReports />
            </TabsContent>
            <TabsContent value="users">
              <UsersPanel />
            </TabsContent>
          </Tabs>
        </>
      ) : (
        <p className="text-sm text-muted-foreground">You are not allowed to view the admin panel.</p>
      )}
    </main>
  );
}
