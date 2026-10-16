"use client";

import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useSession } from "@/lib/session";
import { StatsCards } from "./stats-cards";

// Tab state lives in memory, not the URL. 20c and 20d fill the panels; keep the order.
export function AdminPanel() {
  const { isAdmin } = useSession();

  return (
    <main id="main-content" tabIndex={-1} className="mx-auto flex w-full max-w-5xl flex-1 flex-col gap-6 p-4 focus:outline-none">
      <h1 className="text-2xl font-semibold tracking-tight">Admin</h1>
      {isAdmin ? (
        <>
          <StatsCards />
          <Tabs defaultValue="reports">
            <TabsList>
              <TabsTrigger value="reports">Reports</TabsTrigger>
              <TabsTrigger value="users">Users</TabsTrigger>
            </TabsList>
            <TabsContent value="reports">
              <p className="text-sm text-muted-foreground">Reports</p>
            </TabsContent>
            <TabsContent value="users">
              <p className="text-sm text-muted-foreground">Users</p>
            </TabsContent>
          </Tabs>
        </>
      ) : (
        <p className="text-sm text-muted-foreground">You are not allowed to view the admin panel.</p>
      )}
    </main>
  );
}
