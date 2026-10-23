"use client";

import { QueryClient, QueryClientProvider, useMutationState } from "@tanstack/react-query";

// No retries: api.ts already retries a stale CSRF token, and a retry would spend
// the per-user rate limit. No refetch on focus, for the same reason.
export const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false } },
});

export function QueryProvider({ children }: { children: React.ReactNode }) {
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

// The variables of every in-flight mutation with this key, so each row can show its own
// pending state while others run (one useMutation only tracks its latest call).
export function usePendingVariables<T>(mutationKey: readonly unknown[]): T[] {
  return useMutationState({ filters: { mutationKey, status: "pending" }, select: (m) => m.state.variables as T });
}

// A report or account change moves the admin cards and the Tracker too. Invalidating marks them
// stale, so each refetches when it is next shown; nothing polls.
export const refreshAdminViews = (client: QueryClient) =>
  Promise.all([
    client.invalidateQueries({ queryKey: ["admin-stats"] }),
    client.invalidateQueries({ queryKey: ["tracker"] }),
  ]);
