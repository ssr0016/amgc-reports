"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { HOME } from "@/lib/auth-form";
import { useSession } from "@/lib/session";
import { ProfileMenu } from "./profile-menu";
import { Sidebar } from "./sidebar";

// Client-side guard: the session cookie is not visible to Server Components or
// proxy.ts (plan section 1.2), so nothing private renders until /auth/me answers.
export default function AppLayout({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const { status, signOut } = useSession();
  const [menuOpen, setMenuOpen] = useState(false);

  useEffect(() => {
    if (status !== "unauthenticated") return;
    const path = location.pathname + location.search;
    // Landing on the home page after signing in is the default, so no ?next= for it.
    router.replace(path === HOME ? "/login" : `/login?next=${encodeURIComponent(path)}`);
  }, [status, router]);

  if (status !== "authenticated") {
    return <main className="flex flex-1 items-center justify-center">Loading…</main>;
  }
  return (
    <>
      <a
        href="#main-content"
        className="sr-only focus:not-sr-only focus:absolute focus:top-2 focus:left-2 focus:z-50 focus:rounded-lg focus:border focus:border-ring focus:bg-background focus:px-3 focus:py-2 focus:text-sm"
      >
        Skip to main content
      </a>
      <header className="border-b">
        <div className="flex w-full flex-wrap items-center justify-between gap-x-4 gap-y-2 p-4">
          <div className="flex items-center gap-3">
            <Button
              variant="outline"
              size="sm"
              className="md:hidden"
              aria-expanded={menuOpen}
              aria-controls="main-nav"
              onClick={() => setMenuOpen((open) => !open)}
            >
              Menu
            </Button>
            <Link href={HOME} className="font-semibold tracking-tight">
              AMGCReports
            </Link>
          </div>
          <div className="flex flex-wrap items-center gap-3 text-sm">
            <ProfileMenu />
            <Button variant="outline" size="sm" onClick={signOut}>
              Sign out
            </Button>
          </div>
        </div>
      </header>
      <div className="flex flex-1 flex-col md:flex-row">
        <Sidebar id="main-nav" open={menuOpen} onNavigate={() => setMenuOpen(false)} />
        <div className="flex min-w-0 flex-1 flex-col">{children}</div>
      </div>
    </>
  );
}
