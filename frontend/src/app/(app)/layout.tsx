"use client";

import { useRouter } from "next/navigation";
import { useEffect } from "react";
import { useSession } from "@/lib/session";

// Client-side guard: the session cookie is not visible to Server Components or
// proxy.ts (plan section 1.2), so nothing private renders until /auth/me answers.
export default function AppLayout({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const { status } = useSession();

  useEffect(() => {
    if (status !== "unauthenticated") return;
    const path = location.pathname + location.search;
    router.replace(path === "/" ? "/login" : `/login?next=${encodeURIComponent(path)}`);
  }, [status, router]);

  if (status !== "authenticated") {
    return <main className="flex flex-1 items-center justify-center">Loading…</main>;
  }
  return children;
}
