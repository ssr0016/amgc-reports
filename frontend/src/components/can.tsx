"use client";

import { useSession } from "@/lib/session";
import type { Permission } from "@/lib/permissions";

// Shows its children only when the signed-in user has the permission.
export function Can({
  permission,
  fallback = null,
  children,
}: {
  permission: Permission;
  fallback?: React.ReactNode;
  children: React.ReactNode;
}) {
  const { can } = useSession();
  return can(permission) ? children : fallback;
}
