"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useRef, useState, type RefObject } from "react";
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
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { NativeSelect } from "@/components/ui/native-select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { ErrorRetry } from "@/components/error-retry";
import { Loading } from "@/components/loading";
import { ApiError } from "@/lib/api";
import { errorText } from "@/lib/auth-form";
import { deleteRole, isBuiltIn, type RoleDetail } from "@/lib/roles";
import { useSession } from "@/lib/session";
import { listRoles, listUsers } from "@/lib/users";

// The Roles tab of /admin (D-A21); it brings no landmark or heading of its own.
export function RolesPanel() {
  const { isAdmin, can } = useSession();
  const writable = can("roles:write");
  const [deleting, setDeleting] = useState<RoleDetail | null>(null);
  const [viewing, setViewing] = useState<RoleDetail | null>(null);
  const opener = useRef<HTMLElement | null>(null);
  // The same key as the role dropdowns on the user pages, so a change here reaches them.
  const { data, error, isPending, refetch } = useQuery({
    queryKey: ["roles"],
    queryFn: listRoles,
    enabled: isAdmin,
    staleTime: 5 * 60_000,
  });

  let body: React.ReactNode;
  if (!isAdmin || (error instanceof ApiError && error.status === 403)) {
    body = <p className="text-sm text-muted-foreground">You are not allowed to manage roles.</p>;
  } else if (error) {
    body = <ErrorRetry message={error.message} onRetry={() => refetch()} />;
  } else if (isPending) {
    body = <Loading />;
  } else {
    body = (
      <>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Description</TableHead>
              <TableHead>Users</TableHead>
              <TableHead>Permissions</TableHead>
              {writable && (
                <TableHead>
                  <span className="sr-only">Actions</span>
                </TableHead>
              )}
            </TableRow>
          </TableHeader>
          <TableBody>
            {data.map((r) => {
              const builtIn = isBuiltIn(r.name);
              return (
                <TableRow key={r.id}>
                  <TableCell>
                    {r.name}
                    {builtIn && (
                      <Badge variant="secondary" className="ml-2">
                        Built-in
                      </Badge>
                    )}
                  </TableCell>
                  <TableCell>{r.description || "—"}</TableCell>
                  <TableCell>
                    {r.user_count > 0 ? (
                      <Button
                        variant="link"
                        size="sm"
                        className="h-auto p-0"
                        aria-label={`Show the ${r.user_count} ${r.user_count === 1 ? "user" : "users"} with role ${r.name}`}
                        onClick={(e) => {
                          opener.current = e.currentTarget;
                          setViewing(r);
                        }}
                      >
                        {r.user_count}
                      </Button>
                    ) : (
                      0
                    )}
                  </TableCell>
                  <TableCell>{r.permissions?.length ?? 0}</TableCell>
                  {writable && (
                    <TableCell>
                      <div className="flex flex-wrap items-center gap-2">
                        <Button asChild variant="outline" size="sm">
                          <Link href={`/admin/roles/${r.id}/edit`}>
                            {builtIn ? "View" : "Edit"} <span className="sr-only">{r.name}</span>
                          </Link>
                        </Button>
                        {!builtIn && (
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={(e) => {
                              opener.current = e.currentTarget;
                              setDeleting(r);
                            }}
                          >
                            Delete <span className="sr-only">{r.name}</span>
                          </Button>
                        )}
                      </div>
                    </TableCell>
                  )}
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
        <DeleteRoleDialog role={deleting} roles={data} opener={opener} onClose={() => setDeleting(null)} />
        <RoleUsersDialog role={viewing} opener={opener} onClose={() => setViewing(null)} />
      </>
    );
  }

  return (
    <div className="flex flex-col gap-6">
      {isAdmin && writable && (
        <div className="flex justify-end">
          <Button asChild>
            <Link href="/admin/roles/new">New role</Link>
          </Button>
        </div>
      )}
      {body}
    </div>
  );
}

// A plain Button, not AlertDialogAction: that one closes on click, and a refused delete must be shown.
// A role in use needs a role to move its users to; the server does both in one transaction.
function DeleteRoleDialog({
  role,
  roles,
  opener,
  onClose,
}: {
  role: RoleDetail | null;
  roles: RoleDetail[];
  opener: RefObject<HTMLElement | null>;
  onClose: () => void;
}) {
  const queryClient = useQueryClient();
  const [target, setTarget] = useState("");
  const count = role?.user_count ?? 0;
  const users = `${count} ${count === 1 ? "user" : "users"}`;
  const del = useMutation({
    mutationFn: (r: RoleDetail) => deleteRole(r.id, count > 0 ? Number(target) : undefined),
    onSuccess: (_, r) => {
      const to = roles.find((o) => String(o.id) === target)?.name;
      toast.success(count > 0 ? `Moved ${users} to ${to} and deleted role ${r.name}` : `Deleted role ${r.name}`);
      setTarget("");
      onClose();
      return Promise.all([
        queryClient.invalidateQueries({ queryKey: ["roles"] }),
        queryClient.invalidateQueries({ queryKey: ["users"] }),
      ]);
    },
  });
  const pending = del.isPending;
  const error = del.error ? errorText(del.error, "Could not delete the role. Try again.") : "";

  // The dialog stays mounted, so the choice is cleared here or it would carry over to the next role.
  function close() {
    del.reset();
    setTarget("");
    onClose();
  }

  return (
    <AlertDialog open={role !== null} onOpenChange={(open) => !open && !pending && close()}>
      <AlertDialogContent
        onCloseAutoFocus={(e) => {
          e.preventDefault();
          // After a delete the row is gone; the main content is the next best place.
          (opener.current?.isConnected ? opener.current : document.getElementById("main-content"))?.focus();
        }}
      >
        <AlertDialogHeader>
          <AlertDialogTitle>Delete role {role?.name}?</AlertDialogTitle>
          <AlertDialogDescription>
            {count > 0
              ? `${users} ${count === 1 ? "holds" : "hold"} this role. Move them to another role first; they get that role's permissions at once.`
              : "No account holds this role."}{" "}
            Deleting it cannot be undone.
          </AlertDialogDescription>
        </AlertDialogHeader>
        {count > 0 && (
          <div className="flex flex-col gap-2">
            <Label htmlFor="reassign-to">Move {users} to</Label>
            <NativeSelect
              id="reassign-to"
              value={target}
              disabled={pending}
              onChange={(e) => setTarget(e.target.value)}
            >
              <option value="">Choose a role…</option>
              {/* admin is left out so a whole role is never promoted by one mis-pick (D-A22). */}
              {roles
                .filter((o) => o.id !== role?.id && o.name !== "admin")
                .map((o) => (
                  <option key={o.id} value={o.id}>
                    {o.name}
                  </option>
                ))}
            </NativeSelect>
          </div>
        )}
        {error && (
          <p role="alert" className="text-sm text-destructive">
            {error}
          </p>
        )}
        <AlertDialogFooter>
          <AlertDialogCancel disabled={pending}>Cancel</AlertDialogCancel>
          <Button onClick={() => role && del.mutate(role)} disabled={pending || (count > 0 && !target)}>
            {pending ? "Deleting…" : count > 0 ? "Move users and delete" : "Delete role"}
          </Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}

// The accounts holding a role. The count includes deleted accounts, so they are fetched and marked too.
// shortcut: each request is capped at 100 users; page if a role ever holds more.
function RoleUsersDialog({
  role,
  opener,
  onClose,
}: {
  role: RoleDetail | null;
  opener: RefObject<HTMLElement | null>;
  onClose: () => void;
}) {
  // Under ["users"] so the invalidations after a user or role change refresh it.
  const { data, error, isPending, refetch } = useQuery({
    queryKey: ["users", "role", role?.id],
    queryFn: async () => (await Promise.all([listUsers("", "", role!.id), listUsers("", "deleted", role!.id)])).flat(),
    enabled: role !== null,
  });

  let body: React.ReactNode;
  if (error) {
    body = <ErrorRetry message={error.message} onRetry={() => refetch()} />;
  } else if (isPending) {
    body = <Loading />;
  } else if (data.length === 0) {
    body = <p className="text-sm text-muted-foreground">No account holds this role.</p>;
  } else {
    body = (
      <ul className="flex max-h-80 flex-col gap-2 overflow-y-auto">
        {data.map((u) => (
          <li key={u.id} className="flex flex-wrap items-center gap-x-2 text-sm">
            <span className="font-medium">{u.name}</span>
            <span className="text-muted-foreground">{u.email}</span>
            {u.deleted_at && <Badge variant="secondary">Deleted</Badge>}
          </li>
        ))}
      </ul>
    );
  }

  return (
    <AlertDialog open={role !== null} onOpenChange={(open) => !open && onClose()}>
      <AlertDialogContent
        onCloseAutoFocus={(e) => {
          e.preventDefault();
          opener.current?.focus();
        }}
      >
        <AlertDialogHeader>
          <AlertDialogTitle>Users with role {role?.name}</AlertDialogTitle>
          <AlertDialogDescription>Change a user&apos;s role from their Edit page.</AlertDialogDescription>
        </AlertDialogHeader>
        {body}
        <AlertDialogFooter>
          <AlertDialogCancel>Close</AlertDialogCancel>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
