"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { cn } from "cn";
import { useSession } from "@/lib/session";

// Hiding Admin and Users is a convenience; the pages and the API still enforce the role.
const ITEMS = [
  { href: "/reports", label: "Reports", admin: false, match: (p: string) => p === "/reports" || p.startsWith("/reports/") },
  // Users lives in the Admin panel's Users tab; its new and edit pages belong to Admin too.
  { href: "/admin", label: "Admin", admin: true, match: (p: string) => p === "/admin" || p.startsWith("/admin/") },
];

// Below md the nav is hidden unless `open` (the layout's Menu button); from md up it is always shown.
export function Sidebar({ id, open = false, onNavigate }: { id?: string; open?: boolean; onNavigate?: () => void }) {
  const pathname = usePathname();
  const { isAdmin } = useSession();
  return (
    <nav
      id={id}
      aria-label="Main"
      className={cn("flex-col gap-1 border-b p-4 text-sm md:flex md:w-48 md:shrink-0 md:border-r md:border-b-0", open ? "flex" : "hidden")}
    >
      {ITEMS.filter((item) => isAdmin || !item.admin).map((item) => {
        const active = item.match(pathname);
        return (
          <Link
            key={item.href}
            href={item.href}
            aria-current={active ? "page" : undefined}
            onClick={onNavigate}
            // The underline and weight mark the current page, so it does not rely on colour alone.
            className={cn("rounded-lg px-2 py-1.5 hover:bg-muted", active && "bg-muted font-semibold underline underline-offset-4")}
          >
            {item.label}
          </Link>
        );
      })}
    </nav>
  );
}
