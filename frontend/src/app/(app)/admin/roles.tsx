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
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { ErrorRetry } from "@/components/error-retry";
import { Loading } from "@/components/loading";
import { ApiError } from "@/lib/api";
import { errorText } from "@/lib/auth-form";
import { deleteRole, isBuiltIn, type RoleDetail } from "@/lib/roles";
import { useSession } from "@/lib/session";
import { listRoles } from "@/lib/users";

// The Roles tab of /admin (D-A21); it brings no landmark or heading of its own.
export function RolesPanel() {
  const { isAdmin, can } = useSession();
  const writable = can("roles:write");
  const [deleting, setDeleting] = useState<RoleDetail | null>(null);
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
              const inUse = r.user_count > 0;
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
                  <TableCell>{r.user_count}</TableCell>
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
                          <>
                            <Button
                              variant="outline"
                              size="sm"
                              disabled={inUse}
                              aria-describedby={inUse ? `role-${r.id}-in-use` : undefined}
                              onClick={(e) => {
                                opener.current = e.currentTarget;
                                setDeleting(r);
                              }}
                            >
                              Delete <span className="sr-only">{r.name}</span>
                            </Button>
                            {inUse && (
                              <span id={`role-${r.id}-in-use`} className="text-sm text-muted-foreground">
                                Reassign its {r.user_count} {r.user_count === 1 ? "user" : "users"} first
                              </span>
                            )}
                          </>
                        )}
                      </div>
                    </TableCell>
                  )}
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
        <DeleteRoleDialog role={deleting} opener={opener} onClose={() => setDeleting(null)} />
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
function DeleteRoleDialog({
  role,
  opener,
  onClose,
}: {
  role: RoleDetail | null;
  opener: RefObject<HTMLElement | null>;
  onClose: () => void;
}) {
  const queryClient = useQueryClient();
  const del = useMutation({
    mutationFn: (r: RoleDetail) => deleteRole(r.id),
    onSuccess: (_, r) => {
      toast.success(`Deleted role ${r.name}`);
      onClose();
      return queryClient.invalidateQueries({ queryKey: ["roles"] });
    },
  });
  const pending = del.isPending;
  const error = del.error ? errorText(del.error, "Could not delete the role. Try again.") : "";

  function close() {
    del.reset();
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
          <AlertDialogDescription>No account holds this role. Deleting it cannot be undone.</AlertDialogDescription>
        </AlertDialogHeader>
        {error && (
          <p role="alert" className="text-sm text-destructive">
            {error}
          </p>
        )}
        <AlertDialogFooter>
          <AlertDialogCancel disabled={pending}>Cancel</AlertDialogCancel>
          <Button onClick={() => role && del.mutate(role)} disabled={pending}>
            {pending ? "Deleting…" : "Delete role"}
          </Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
