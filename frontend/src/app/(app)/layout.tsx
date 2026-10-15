"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect } from "react";
import { Avatar } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import { HOME } from "@/lib/auth-form";
import { useSession } from "@/lib/session";

// Client-side guard: the session cookie is not visible to Server Components or
// proxy.ts (plan section 1.2), so nothing private renders until /auth/me answers.
export default function AppLayout({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const { status, user, isAdmin, signOut } = useSession();

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
        <div className="mx-auto flex w-full max-w-5xl items-center justify-between gap-4 p-4">
          <Link href={HOME} className="font-semibold tracking-tight">
            AMGCReports
          </Link>
          <div className="flex items-center gap-3 text-sm">
            {isAdmin && (
              <Link href="/admin/users" className="hover:underline">
                Users
              </Link>
            )}
            <Avatar name={user?.name ?? ""} />
            <span title={user?.email}>
              {user?.name} · {user?.role || "no role"}
            </span>
            <Button variant="outline" size="sm" onClick={signOut}>
              Sign out
            </Button>
          </div>
        </div>
      </header>
      {children}
    </>
  );
}
