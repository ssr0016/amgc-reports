"use client";

import dynamic from "next/dynamic";
import { useSearchParams } from "next/navigation";
import { Card } from "@/components/ui/card";
import { AnalyticsSkeleton } from "@/components/loading-skeletons";
import { adminSections } from "@/lib/admin-navigation";
import { useSession } from "@/lib/session";
import { reportsTheme } from "@/components/reports-dashboard";
import { AdminReports } from "./admin-reports";
import { AuditLogs } from "./audit-logs";
import { RoleStats, RolesPanel } from "./roles";
import { StatsCards } from "./stats-cards";
import { Tracker } from "./tracker";
import { UsersPanel } from "./users/users-list";
import { PageHeader, PageShell } from "@/components/page-shell";

// Recharts only downloads when someone opens the Analytics tab.
const Analytics = dynamic(() => import("./analytics").then((m) => m.Analytics), { loading: () => <AnalyticsSkeleton /> });

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
    <PageShell className={`${analyticsTab ? "analytics-admin-layout" : ""} ${tab === "reports" ? reportsTheme : ""} max-w-7xl`}>
      <PageHeader title={<>Admin{tab && ` / ${tabs.find((t) => t.value === tab)?.label}`}</>} />
      {canAdmin && tab ? (
        <>
          {/* Tracker has its own month-scoped summary, so the global cards would duplicate or contradict it.
              Logs is for finding events, not a summary, so it shows no cards. */}
          {tab === "roles" ? <RoleStats /> : !["reports", "tracker", "logs"].includes(tab) && <StatsCards users={tab === "users"} />}
          <section aria-label={tabs.find((t) => t.value === tab)?.label} className={`${analyticsTab ? "analytics-admin-section" : ""} min-w-0`}>
            {tab === "reports" || tab === "logs" ? panels[tab] : <Card className={`${analyticsTab ? "analytics-admin-card" : ""} p-4 sm:p-6`}>{panels[tab]}</Card>}
          </section>
        </>
      ) : (
        <p className="text-sm text-muted-foreground">You are not allowed to view the admin panel.</p>
      )}
    </PageShell>
  );
}
