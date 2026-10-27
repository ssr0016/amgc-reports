"use client";

import { useIsFetching } from "@tanstack/react-query";
import { usePathname } from "next/navigation";
import { useEffect, useState } from "react";

function isInternalNavigation(anchor: HTMLAnchorElement) {
  if (anchor.target === "_blank" || anchor.hasAttribute("download")) return false;
  if (anchor.origin !== window.location.origin) return false;
  if (anchor.pathname === window.location.pathname && anchor.search === window.location.search) return false;
  return !anchor.hash || anchor.pathname !== window.location.pathname;
}

export function GlobalActivityIndicator() {
  const pathname = usePathname();
  const fetching = useIsFetching() > 0;
  const [navigationPath, setNavigationPath] = useState<string | null>(null);
  const navigating = navigationPath === pathname;

  useEffect(() => {
    const start = (event: MouseEvent) => {
      if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
      const target = event.target instanceof Element ? event.target.closest("a") : null;
      if (target instanceof HTMLAnchorElement && isInternalNavigation(target)) setNavigationPath(pathname);
    };
    document.addEventListener("click", start, true);
    return () => document.removeEventListener("click", start, true);
  }, [pathname]);

  return (
    <>
      {navigating && <div data-testid="route-progress" aria-hidden="true" className="fixed inset-x-0 top-0 z-[100] h-0.5 bg-primary motion-safe:animate-pulse" />}
      {fetching && (
        <div data-testid="background-fetch-indicator" role="status" className="fixed top-0 right-3 z-[101] rounded-b-md bg-primary px-2 py-0.5 text-[10px] font-medium text-primary-foreground shadow-sm">
          Updating
        </div>
      )}
    </>
  );
}
