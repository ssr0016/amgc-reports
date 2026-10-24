"use client";

import Link from "next/link";
import { Avatar } from "@/components/ui/avatar";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { useSession } from "@/lib/session";

// The avatar and name are one button; e2e/auth.spec.ts looks for the "{name} · {role}" text.
export function ProfileMenu() {
  const { user } = useSession();
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        title={user?.email}
        className="flex cursor-pointer items-center gap-2 rounded-lg text-sm focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none"
      >
        <Avatar name={user?.name ?? ""} src={user?.avatar_url} />
        <span>
          {user?.name} · {user?.role || "no role"}
        </span>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem asChild>
          <Link href="/account/profile">Profile picture</Link>
        </DropdownMenuItem>
        <DropdownMenuItem asChild>
          <Link href="/account/password">Change password</Link>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
