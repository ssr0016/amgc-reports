"use client";

import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { getCurrentUser, logout, type Me } from "@/lib/auth";

export default function Home() {
  const router = useRouter();
  const [user, setUser] = useState<Me | null>(null);

  useEffect(() => {
    getCurrentUser().then((me) => (me ? setUser(me) : router.replace("/login")));
  }, [router]);

  async function onLogout() {
    await logout();
    router.replace("/login");
  }

  // Nothing private renders until the API confirms the session.
  if (!user) return <main className="flex flex-1 items-center justify-center">Loading…</main>;

  return (
    <main className="flex flex-1 flex-col items-center justify-center gap-4">
      <h1 className="text-3xl font-semibold tracking-tight">AMGCReports</h1>
      <p>
        {user.name} ({user.email}) · {user.role || "no role"}
      </p>
      <button onClick={onLogout} className="rounded border border-zinc-400 px-3 py-2">
        Sign out
      </button>
    </main>
  );
}
