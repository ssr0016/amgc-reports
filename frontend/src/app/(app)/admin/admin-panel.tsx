"use client";

import { useSearchParams } from "next/navigation";
import { Card } from "@/components/ui/card";
import { adminSections } from "@/lib/admin-navigation";
import { useSession } from "@/lib/session";
import { reportsTheme } from "@/components/reports-dashboard";
import { AdminReports } from "./admin-reports";
import { Analytics } from "./analytics";
import { AuditLogs } from "./audit-logs";
import { RolesPanel } from "./roles";
import { StatsCards } from "./stats-cards";
import { Tracker } from "./tracker";
import { UsersPanel } from "./users/users-list";

// Sidebar links keep the existing ?tab= URLs and permission fallback order.
export function AdminPanel() {
  const { isAdmin, can, canAdmin } = useSession();
  const param = useSearchParams().get("tab");

  // Each tab shows only to those who may use it (37b); the server still enforces every route.
  const tabs = adminSections(can, isAdmin);
  const panels: Record<string, React.ReactNode> = {
    reports: <AdminReports />,
    analytics: <Analytics />,
    tracker: <Tracker />,
    users: <UsersPanel />,
    roles: <RolesPanel />,
    logs: <AuditLogs />,
  };
  // A missing, unknown or not-allowed ?tab= lands on the first allowed tab.
  const tab = tabs.find((t) => t.value === param)?.value ?? tabs[0]?.value;
  const analyticsTab = tab === "analytics";

  return (
    <main id="main-content" tabIndex={-1} className={`${analyticsTab ? "analytics-admin-layout" : ""} ${tab === "reports" ? reportsTheme : ""} mx-auto flex w-full max-w-7xl flex-1 flex-col gap-6 p-4 sm:p-6 focus:outline-none`}>
      <h1 className={`${analyticsTab ? "analytics-admin-heading" : ""} text-2xl font-medium tracking-tight`}>Admin{tab && ` / ${tabs.find((t) => t.value === tab)?.label}`}</h1>
      {canAdmin && tab ? (
        <>
          {tab !== "reports" && <StatsCards users={tab === "users"} />}
          <section aria-label={tabs.find((t) => t.value === tab)?.label} className={`${analyticsTab ? "analytics-admin-section" : ""} min-w-0`}>
            {tab === "reports" ? panels[tab] : <Card className={`${analyticsTab ? "analytics-admin-card" : ""} p-4 sm:p-6`}>{panels[tab]}</Card>}
          </section>
        </>
      ) : (
        <p className="text-sm text-muted-foreground">You are not allowed to view the admin panel.</p>
      )}
    </main>
  );
}
