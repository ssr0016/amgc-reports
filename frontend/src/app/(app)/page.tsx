"use client";

import { Button } from "@/components/ui/button";
import { useSession } from "@/lib/session";

export default function Home() {
  const { user, signOut } = useSession();

  return (
    <main className="flex flex-1 flex-col items-center justify-center gap-4">
      <h1 className="text-3xl font-semibold tracking-tight">AMGCReports</h1>
      <p>
        {user?.name} ({user?.email}) · {user?.role || "no role"}
      </p>
      <Button variant="outline" onClick={signOut}>
        Sign out
      </Button>
    </main>
  );
}
