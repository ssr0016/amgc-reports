"use client";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

// No retries: api.ts already retries a stale CSRF token, and a retry would spend
// the per-user rate limit. No refetch on focus, for the same reason.
export const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false } },
});

export function QueryProvider({ children }: { children: React.ReactNode }) {
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

// A report or account change moves the admin cards and the Tracker too. Invalidating marks them
// stale, so each refetches when it is next shown; nothing polls.
export const refreshAdminViews = (client: QueryClient) =>
  Promise.all([
    client.invalidateQueries({ queryKey: ["admin-stats"] }),
    client.invalidateQueries({ queryKey: ["tracker"] }),
  ]);
