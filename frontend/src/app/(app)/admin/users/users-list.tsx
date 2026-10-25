"use client";

import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { Search, UserPlus } from "lucide-react";
import { Card } from "@/components/ui/card";
import { useEffect, useState } from "react";
import { toast } from "sonner";
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Avatar } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { NativeSelect } from "@/components/ui/native-select";
import { UserStatusBadge } from "@/components/user-status-badge";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Textarea } from "@/components/ui/textarea";
import { ApiError } from "@/lib/api";
import { useSession } from "@/lib/session";
import { deleteUser, listRoles, listUsers, restoreUser, type User } from "@/lib/users";
import { STATUS_LABEL, USER_STATUSES } from "@/lib/user-status";
import { Loading } from "@/components/loading";
import { ErrorRetry } from "@/components/error-retry";
import { errorText } from "@/lib/auth-form";
import { refreshAdminViews, usePendingVariables } from "@/lib/query";

// The list endpoint has no pages, so it returns at most this many.
const LIMIT = 100;

// The Users tab of /admin; it brings no landmark or heading of its own.
export function UsersPanel() {
  const { isAdmin, can, user } = useSession();
  // users:list reads the list (37b); every change, and the roles it names, stay admin-only.
  const allowed = can("users:list");
  const [search, setSearch] = useState("");
  // shortcut: the role filter only narrows the loaded rows (at most LIMIT), upgrade to a server-side filter when users outgrow LIMIT.
  const [role, setRole] = useState("");
  // Unlike the role, the status filter is asked of the server ("" means every status, deleted users left out).
  const [status, setStatus] = useState("");
  const [email, setEmail] = useState("");
  const [deleting, setDeleting] = useState<User | null>(null);
  const queryClient = useQueryClient();

  // Restore is reversible, so it does not ask first. A row stays busy until the lists refetch.
  const restore = useMutation({
    mutationKey: ["restore-user"],
    mutationFn: (u: User) => restoreUser(u.id),
    onSuccess: (_, u) => {
      toast.success(`${u.name} restored`);
      return Promise.all([
        queryClient.invalidateQueries({ queryKey: ["users"] }),
        queryClient.invalidateQueries({ queryKey: ["reports"] }),
        queryClient.invalidateQueries({ queryKey: ["user", u.id] }),
        refreshAdminViews(queryClient),
      ]);
    },
    onError: (err) => toast.error(errorText(err, "Could not restore the user. Try again.")),
  });
  const restoring = usePendingVariables<User>(["restore-user"]).map((u) => u.id);

  // The box waits 300 ms after the last key, so typing does not spend the read rate limit one request per letter.
  useEffect(() => {
    const timer = setTimeout(() => setEmail(search.trim()), 300);
    return () => clearTimeout(timer);
  }, [search]);

  const { data, error, isPending, refetch } = useQuery({
    queryKey: ["users", email, status],
    queryFn: () => listUsers(email, status),
    enabled: allowed,
    placeholderData: keepPreviousData,
    staleTime: 30_000,
  });
  // If this fails, or for a non-admin, the list still renders, with the role shown as #id.
  const roles = useQuery({ queryKey: ["roles"], queryFn: listRoles, enabled: isAdmin, staleTime: 5 * 60_000 });
  const roleName = new Map(roles.data?.map((r) => [r.id, r.name]));
  const rows = (data ?? []).filter((u) => !role || u.role_id === Number(role));

  let body: React.ReactNode;
  if (!allowed || (error instanceof ApiError && error.status === 403)) {
    body = <p className="text-sm text-muted-foreground">You are not allowed to manage users.</p>;
  } else if (error) {
    body = <ErrorRetry message={error.message} onRetry={() => refetch()} />;
  } else if (isPending) {
    body = <Loading />;
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
              <TableHead>Status</TableHead>
              <TableHead>Area</TableHead>
              <TableHead>Church</TableHead>
              {isAdmin && (
                <TableHead>
                  <span className="sr-only">Actions</span>
                </TableHead>
              )}
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((u) => (
              <TableRow key={u.id}>
                <TableCell>
                  <Avatar name={u.name} src={u.avatar_url} className="size-10 bg-secondary! text-secondary-foreground! cursor-default" />
                </TableCell>
                <TableCell className="font-medium">
                  {u.name}
                  {u.id === user?.user_id && (
                    <Badge variant="secondary" className="ml-2">
                      You
                    </Badge>
                  )}
                </TableCell>
                <TableCell>{u.email}</TableCell>
                <TableCell><Badge variant="secondary">{roleName.get(u.role_id) ?? `#${u.role_id}`}</Badge></TableCell>
                <TableCell>
                  <UserStatusBadge status={u.status} />
                  {u.deleted_at && (
                    <Badge variant="destructive" className="ml-2">
                      Deleted
                    </Badge>
                  )}
                </TableCell>
                <TableCell>{u.area_assignment || "—"}</TableCell>
                <TableCell>{u.church_name || "—"}</TableCell>
                {isAdmin && (
                  <TableCell>
                    <div className="flex justify-end gap-2">
                      <Button asChild variant="outline" size="sm" className="min-h-11">
                        <Link href={`/admin/users/${u.id}/edit`}>
                          Edit{" "}
                          <span className="sr-only">{u.name}</span>
                        </Link>
                      </Button>
                      {u.deleted_at ? (
                        <Button variant="outline" size="sm" className="min-h-11" disabled={restoring.includes(u.id)} onClick={() => restore.mutate(u)}>
                          Restore <span className="sr-only">{u.name}</span>
                        </Button>
                      ) : (
                        <Button
                          variant="outline"
                          size="sm"
                          className="min-h-11"
                          disabled={u.id === user?.user_id}
                          aria-describedby={u.id === user?.user_id ? "delete-self-note" : undefined}
                          onClick={() => setDeleting(u)}
                        >
                          Delete <span className="sr-only">{u.name}</span>
                        </Button>
                      )}
                    </div>
                  </TableCell>
                )}
              </TableRow>
            ))}
          </TableBody>
        </Table>
        <p id="delete-self-note" className="sr-only">
          You cannot delete your own account.
        </p>
        <DeleteUserDialog user={deleting} onClose={() => setDeleting(null)} />
        {data.length >= LIMIT && (
          <p className="mt-4 text-sm text-muted-foreground">Showing the first <span className="font-mono tabular-nums">{LIMIT}</span>. Search by email to narrow it.</p>
        )}
      </>
    );
  }

  return (
    <div className="account-directory flex min-w-0 flex-col gap-6">
      {allowed && (
        <>
          <header className="flex flex-wrap items-start justify-between gap-4">
            <div>
              <h2 className="text-2xl font-semibold tracking-tight">Users</h2>
              <p className="mt-2 text-sm text-muted-foreground">Manage church workers and their account access.</p>
            </div>
            {isAdmin && (
              <Button asChild className="min-h-11 px-4">
                <Link href="/admin/users/new"><UserPlus aria-hidden="true" />New user</Link>
              </Button>
            )}
          </header>
          <Card className="gap-0 rounded-lg py-0">
            <div className="grid items-end gap-4 border-b p-4 sm:grid-cols-2 lg:grid-cols-3">
              <div className="flex min-w-0 flex-col gap-2">
                <Label htmlFor="email-search">Email</Label>
                <div className="relative">
                  <Search aria-hidden="true" className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
                  <Input
                    className="pl-9"
                    id="email-search"
                    type="search"
                    placeholder="Search by email"
                    value={search}
                    onChange={(e) => setSearch(e.target.value)}
                  />
                </div>
              </div>
              {isAdmin && (
                <div className="flex min-w-0 flex-col gap-2">
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
              )}
              <div className="flex min-w-0 flex-col gap-2">
                <Label htmlFor="status-filter">Status</Label>
                <NativeSelect id="status-filter" value={status} onChange={(e) => setStatus(e.target.value)}>
                  <option value="">All statuses</option>
                  {USER_STATUSES.map((s) => (
                    <option key={s} value={s}>
                      {STATUS_LABEL[s]}
                    </option>
                  ))}
                  <option value="deleted">Deleted</option>
                </NativeSelect>
              </div>
            </div>
            <div className="min-w-0 p-4">{body}</div>
          </Card>
        </>
      )}
      {!allowed && body}
    </div>
  );
}

// Asks first (D-A11). A plain Button, not AlertDialogAction: that one closes on click, and a failed delete must be shown.
function DeleteUserDialog({ user, onClose }: { user: User | null; onClose: () => void }) {
  const queryClient = useQueryClient();
  const [reason, setReason] = useState("");
  const del = useMutation({
    mutationFn: (u: User) => deleteUser(u.id, reason),
    onSuccess: (_, u) => {
      toast.success(`${u.name} deleted`);
      setReason("");
      onClose();
      return Promise.all([
        queryClient.invalidateQueries({ queryKey: ["users"] }),
        queryClient.invalidateQueries({ queryKey: ["reports"] }),
        queryClient.invalidateQueries({ queryKey: ["user", u.id] }),
        refreshAdminViews(queryClient),
      ]);
    },
  });
  const pending = del.isPending;
  const error = del.error ? errorText(del.error, "Could not delete the user. Try again.") : "";

  function close() {
    setReason("");
    del.reset();
    onClose();
  }

  return (
    <AlertDialog open={user !== null} onOpenChange={(open) => !open && !pending && close()}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Delete {user?.name}?</AlertDialogTitle>
          <AlertDialogDescription>
            This will soft-delete {user?.name} ({user?.email}). The user cannot sign in. The user is hidden from the
            active list, but their reports stay. You can restore them later.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <div className="flex flex-col gap-2">
          <Label htmlFor="delete-reason">Reason (optional)</Label>
          <Textarea
            id="delete-reason"
            maxLength={500}
            value={reason}
            disabled={pending}
            onChange={(e) => setReason(e.target.value)}
          />
        </div>
        {error && (
          <p role="alert" className="text-sm text-destructive">
            {error}
          </p>
        )}
        <AlertDialogFooter>
          <AlertDialogCancel disabled={pending}>Cancel</AlertDialogCancel>
          <Button onClick={() => user && del.mutate(user)} disabled={pending}>
            {pending ? "Deleting…" : "Delete user"}
          </Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
