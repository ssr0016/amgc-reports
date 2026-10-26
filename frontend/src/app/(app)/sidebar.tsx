"use client";

import Link from "next/link";
import { usePathname, useSearchParams } from "next/navigation";
import { BarChart3, ClipboardList, KeyRound, ListChecks, ScrollText, ShieldCheck, UserRound, Users } from "lucide-react";
import { cn } from "cn";
import { adminSections } from "@/lib/admin-navigation";
import { useSession } from "@/lib/session";

const ADMIN_ICONS: Record<string, typeof ClipboardList> = {
  reports: ClipboardList, analytics: BarChart3, tracker: ListChecks, users: Users, roles: ShieldCheck, logs: ScrollText,
};

export function Sidebar({ label = "Main", onNavigate }: { label?: string; onNavigate?: () => void }) {
  const pathname = usePathname();
  const params = useSearchParams();
  const { can, isAdmin, canAdmin } = useSession();
  const sections = adminSections(can, isAdmin);
  const inAdmin = pathname === "/admin" || pathname.startsWith("/admin/");
  const requested = pathname.startsWith("/admin/users/") ? "users" : pathname.startsWith("/admin/roles/") ? "roles" : params.get("tab");
  const activeAdmin = sections.find((section) => section.value === requested)?.value ?? sections[0]?.value;
  const linkClass = (active: boolean) => cn(
    "flex min-h-11 items-center gap-3 rounded-sm px-3 py-2 text-sm transition-colors hover:bg-accent hover:text-foreground",
    active ? "bg-accent font-medium text-primary" : "text-muted-foreground",
  );
  return (
    <nav
      aria-label={label}
      className="flex min-h-0 flex-1 flex-col gap-6 p-4"
    >
      {can("reports:read") && (
        <Link
          href="/reports"
          aria-current={pathname.startsWith("/reports") ? "page" : undefined}
          onClick={onNavigate}
          className={linkClass(pathname.startsWith("/reports"))}
        >
          <ClipboardList className="size-4" aria-hidden="true" />
          Community Reports
        </Link>
      )}
      {canAdmin && sections.length > 0 && (
        <section aria-labelledby={`${label}-admin`} className="flex flex-col gap-1">
          <h2 id={`${label}-admin`} className="mb-1 px-3 text-xs font-medium text-muted-foreground">Admin</h2>
          {sections.map((section) => {
            const title = section.value === "reports" ? "All Reports" : section.label;
            const active = inAdmin && section.value === activeAdmin;
            const Icon = ADMIN_ICONS[section.value];
            return (
              <Link key={section.value} href={`/admin?tab=${section.value}`} aria-label={`Admin ${title}`} replace={inAdmin} aria-current={active ? "page" : undefined} onClick={onNavigate} className={linkClass(active)}>
                <Icon className="size-4" aria-hidden="true" />{title}
              </Link>
            );
          })}
        </section>
      )}
      <section aria-labelledby={`${label}-settings`} className="mt-auto flex flex-col gap-1 border-t pt-4">
        <h2 id={`${label}-settings`} className="mb-1 px-3 text-xs font-medium text-muted-foreground">Settings</h2>
        <Link href="/account/profile" aria-current={pathname === "/account/profile" ? "page" : undefined} onClick={onNavigate} className={linkClass(pathname === "/account/profile")}>
          <UserRound className="size-4" aria-hidden="true" />Profile
        </Link>
        <Link href="/account/password" aria-current={pathname === "/account/password" ? "page" : undefined} onClick={onNavigate} className={linkClass(pathname === "/account/password")}>
          <KeyRound className="size-4" aria-hidden="true" />Change Password
        </Link>
      </section>
    </nav>
  );
}
