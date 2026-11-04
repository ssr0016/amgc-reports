"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Bell } from "lucide-react";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { listUnreadNotifications, markNotificationRead, type Notification } from "@/lib/notifications";
import { useSession } from "@/lib/session";

const queryKey = ["notifications", "unread"] as const;

export function NotificationBell() {
  const { can } = useSession();
  const allowed = can("reports:approve");
  const router = useRouter();
  const queryClient = useQueryClient();
  const query = useQuery({
    queryKey,
    queryFn: listUnreadNotifications,
    enabled: allowed,
    refetchInterval: 30_000,
  });
  const markRead = useMutation({
    mutationFn: markNotificationRead,
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey });
    },
  });

  if (!allowed) return null;

  const notifications = query.data ?? [];
  const unreadCount = notifications.length >= 50 ? "at least 50" : notifications.length;
  const label = query.isPending ? "Notifications" : `Notifications, ${notifications.length ? `${unreadCount} unread` : "no unread"}`;
  const select = (item: Notification) => {
    markRead.mutate(item.id, { onSuccess: () => router.push("/admin?tab=reports") });
  };

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" className="relative" aria-label={label} aria-busy={query.isPending}>
          <Bell aria-hidden="true" />
          {notifications.length > 0 && <span data-unread-dot aria-hidden="true" className="absolute right-2 top-2 size-2 rounded-full bg-destructive" />}
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="max-h-[min(24rem,var(--radix-dropdown-menu-content-available-height))] w-80 max-w-[calc(100vw-2rem)] overflow-y-auto">
        {query.isPending && <p className="px-2 py-3 text-sm text-muted-foreground">Loading notifications…</p>}
        {query.isError && <p className="px-2 py-3 text-sm text-destructive">Unable to load notifications.</p>}
        {!query.isPending && !query.isError && notifications.length === 0 && (
          <p className="px-2 py-3 text-sm text-muted-foreground">No unread notifications</p>
        )}
        {!query.isPending && !query.isError && notifications.map((item) => (
          <DropdownMenuItem key={item.id} onSelect={() => select(item)}>
            <span className="whitespace-normal">{item.message}</span>
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
