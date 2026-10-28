"use client";

import Link from "next/link";
import { Avatar } from "@/components/ui/avatar";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { useSession } from "@/lib/session";

// The avatar and name are one button, avatar-only below sm so the header stays one row; e2e/auth.spec.ts looks for the "{name} · {role}" text.
export function ProfileMenu({ onSignOut }: { onSignOut: () => void }) {
  const { user } = useSession();
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        title={user?.email}
        aria-label={`${user?.name} · ${user?.role || "no role"}`}
        className="flex min-h-11 min-w-11 max-w-full cursor-pointer items-center gap-2 rounded-sm px-2 text-sm hover:bg-accent focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none"
      >
        <Avatar name={user?.name ?? ""} src={user?.avatar_url} />
        <span className="hidden max-w-64 truncate sm:inline">
          {user?.name} · {user?.role || "no role"}
        </span>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem asChild>
          <Link href="/account/profile">Profile</Link>
        </DropdownMenuItem>
        <DropdownMenuItem asChild>
          <Link href="/account/password">Change Password</Link>
        </DropdownMenuItem>
        <DropdownMenuItem onClick={onSignOut}>Sign Out</DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
