"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { Suspense, useEffect, useRef, useState } from "react";
import { Menu, X } from "lucide-react";
import { Dialog } from "radix-ui";
import { Button } from "@/components/ui/button";
import { HOME } from "@/lib/auth-form";
import { useSession } from "@/lib/session";
import { ProfileMenu } from "./profile-menu";
import { Sidebar } from "./sidebar";

// Client-side guard: the API's session cookies cannot tell proxy.ts whether a session is
// live (plan section 1.2), so nothing private renders until /auth/me answers. proxy.ts
// only turns away visitors without the session's signed_in hint.
export default function AppLayout({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const { status, signOut } = useSession();
  const [menuOpen, setMenuOpen] = useState(false);
  // A chosen sign-out gets no ?next=: the next person to sign in may be someone else (a user sent to /admin).
  const signingOut = useRef(false);

  useEffect(() => {
    const desktop = window.matchMedia("(min-width: 1024px)");
    const closeOnDesktop = () => { if (desktop.matches) setMenuOpen(false); };
    desktop.addEventListener("change", closeOnDesktop);
    return () => desktop.removeEventListener("change", closeOnDesktop);
  }, []);

  useEffect(() => {
    if (status !== "unauthenticated") return;
    const path = location.pathname + location.search;
    const keepPath = !signingOut.current && path !== HOME;
    signingOut.current = false;
    // Landing on the home page after signing in is the default, so no ?next= for it.
    router.replace(keepPath ? `/login?next=${encodeURIComponent(path)}` : "/login");
  }, [status, router]);

  if (status !== "authenticated") {
    return <main className="flex flex-1 items-center justify-center">Loading…</main>;
  }
  return (
    <>
      <a
        href="#main-content"
        className="sr-only cursor-pointer focus:not-sr-only focus:absolute focus:top-2 focus:left-2 focus:z-50 focus:rounded-lg focus:border focus:border-ring focus:bg-background focus:px-3 focus:py-2 focus:text-sm"
      >
        Skip to main content
      </a>
      <Dialog.Root open={menuOpen} onOpenChange={setMenuOpen}>
      <header className="app-top-nav border-b bg-background">
        <div className="flex w-full flex-wrap items-center justify-between gap-x-4 gap-y-2 p-4">
          <div className="flex items-center gap-3">
            <Dialog.Trigger asChild><Button
              variant="outline"
              size="icon"
              className="lg:hidden"
              aria-expanded={menuOpen}
              aria-controls="mobile-navigation"
            >
              <Menu aria-hidden="true" /><span className="sr-only">Menu</span>
            </Button></Dialog.Trigger>
            <Link href={HOME} className="flex min-h-11 items-center font-medium tracking-tight">
              AMGCReports
            </Link>
          </div>
          <div className="flex flex-wrap items-center gap-3 text-sm">
            <ProfileMenu />
            <Button
              variant="outline"
              size="sm"
              onClick={() => {
                signingOut.current = true;
                void signOut();
              }}
            >
              Sign out
            </Button>
          </div>
        </div>
      </header>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-50 bg-black/60 backdrop-blur-sm data-open:animate-in data-open:fade-in-0 data-closed:animate-out data-closed:fade-out-0" />
        <Dialog.Content id="mobile-navigation" aria-describedby={undefined} className="fixed inset-y-0 left-0 z-50 flex w-[min(20rem,calc(100%-1rem))] flex-col overflow-y-auto border-r bg-background p-2 shadow-xl outline-none duration-200 data-open:animate-in data-open:slide-in-from-left data-closed:animate-out data-closed:slide-out-to-left">
          <div className="flex items-center justify-between border-b p-2">
            <Dialog.Title className="px-3 font-medium">AMGCReports navigation</Dialog.Title>
            <Dialog.Close asChild><Button variant="ghost" size="icon" aria-label="Close navigation"><X aria-hidden="true" /></Button></Dialog.Close>
          </div>
          <Suspense fallback={null}><Sidebar label="Mobile" onNavigate={() => setMenuOpen(false)} /></Suspense>
        </Dialog.Content>
      </Dialog.Portal>
      </Dialog.Root>
      <div className="flex min-w-0 flex-1">
        <aside className="hidden w-60 shrink-0 border-r lg:flex">
          <div className="sticky top-0 flex h-dvh max-h-dvh w-full overflow-y-auto">
            <Suspense fallback={null}><Sidebar /></Suspense>
          </div>
        </aside>
        <div className="flex min-w-0 flex-1 flex-col">{children}</div>
      </div>
    </>
  );
}
