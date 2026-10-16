"use client";

import { keepPreviousData, useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { useEffect, useState } from "react";
import { Avatar } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { NativeSelect } from "@/components/ui/native-select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { ApiError } from "@/lib/api";
import { useSession } from "@/lib/session";
import { listRoles, listUsers } from "@/lib/users";

// The list endpoint has no pages, so it returns at most this many.
const LIMIT = 100;

// The Users tab of /admin; it brings no landmark or heading of its own.
export function UsersPanel() {
  const { isAdmin, user } = useSession();
  const [search, setSearch] = useState("");
  // shortcut: the role filter only narrows the loaded rows (at most LIMIT), upgrade to a server-side filter when users outgrow LIMIT.
  const [role, setRole] = useState("");
  const [email, setEmail] = useState("");

  // The box waits 300 ms after the last key, so typing does not spend the read rate limit one request per letter.
  useEffect(() => {
    const timer = setTimeout(() => setEmail(search.trim()), 300);
    return () => clearTimeout(timer);
  }, [search]);

  const { data, error, isPending } = useQuery({
    queryKey: ["users", email],
    queryFn: () => listUsers(email),
    enabled: isAdmin,
    placeholderData: keepPreviousData,
    staleTime: 30_000,
  });
  // If this fails the list still renders, with the role shown as #id.
  const roles = useQuery({ queryKey: ["roles"], queryFn: listRoles, enabled: isAdmin, staleTime: 5 * 60_000 });
  const roleName = new Map(roles.data?.map((r) => [r.id, r.name]));
  const rows = (data ?? []).filter((u) => !role || u.role_id === Number(role));

  let body: React.ReactNode;
  if (!isAdmin || (error instanceof ApiError && error.status === 403)) {
    body = <p className="text-sm text-muted-foreground">You are not allowed to manage users.</p>;
  } else if (error) {
    body = (
      <p role="alert" className="text-sm text-destructive">
        {error.message}
      </p>
    );
  } else if (isPending) {
    body = <p role="status" className="text-sm text-muted-foreground">Loading…</p>;
  } else if (rows.length === 0) {
    body = <p role="status" className="text-sm text-muted-foreground">No users match.</p>;
  } else {
    body = (
      <>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>
                <span className="sr-only">Avatar</span>
              </TableHead>
              <TableHead>Name</TableHead>
              <TableHead>Email</TableHead>
              <TableHead>Role</TableHead>
              <TableHead>Area</TableHead>
              <TableHead>Church</TableHead>
              <TableHead>
                <span className="sr-only">Actions</span>
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((u) => (
              <TableRow key={u.id}>
                <TableCell>
                  <Avatar name={u.name} />
                </TableCell>
                <TableCell>
                  {u.name}
                  {u.id === user?.user_id && (
                    <Badge variant="secondary" className="ml-2">
                      You
                    </Badge>
                  )}
                </TableCell>
                <TableCell>{u.email}</TableCell>
                <TableCell>{roleName.get(u.role_id) ?? `#${u.role_id}`}</TableCell>
                <TableCell>{u.area_assignment || "—"}</TableCell>
                <TableCell>{u.church_name || "—"}</TableCell>
                <TableCell>
                  <Button asChild variant="outline" size="sm">
                    <Link href={`/admin/users/${u.id}/edit`}>
                      Edit{" "}
                      <span className="sr-only">{u.name}</span>
                    </Link>
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
        {data.length >= LIMIT && (
          <p className="text-sm text-muted-foreground">Showing the first {LIMIT}. Search by email to narrow it.</p>
        )}
      </>
    );
  }

  return (
    <div className="flex flex-col gap-6">
      {isAdmin && (
        <div className="flex flex-wrap items-end justify-between gap-4">
          <div className="flex flex-wrap items-end gap-4">
            <div className="flex w-64 flex-col gap-2">
              <Label htmlFor="email-search">Email</Label>
              <Input
                id="email-search"
                type="search"
                placeholder="Search by email"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
              />
            </div>
            <div className="flex w-40 flex-col gap-2">
              <Label htmlFor="role-filter">Role</Label>
              <NativeSelect id="role-filter" value={role} onChange={(e) => setRole(e.target.value)}>
                <option value="">All roles</option>
                {roles.data?.map((r) => (
                  <option key={r.id} value={r.id}>
                    {r.name}
                  </option>
                ))}
              </NativeSelect>
            </div>
          </div>
          <Button asChild>
            <Link href="/admin/users/new">New user</Link>
          </Button>
        </div>
      )}
      {body}
    </div>
  );
}
