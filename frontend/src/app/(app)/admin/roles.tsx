"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { KeyRound, Loader2, Lock, Plus, Shield, ShieldPlus } from "lucide-react";
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
import { Card } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { NativeSelect } from "@/components/ui/native-select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { ErrorRetry } from "@/components/error-retry";
import { KpiSkeleton, TableSkeleton } from "@/components/loading-skeletons";
import { EmptyState } from "@/components/empty-state";
import { pageOf, Pagination } from "@/components/pagination";
import { panelCard } from "@/components/panel-section";
import { StatCard } from "@/components/stat-card";
import { ApiError } from "@/lib/api";
import { errorText } from "@/lib/auth-form";
import { deleteRole, isBuiltIn, type RoleDetail } from "@/lib/roles";
import { useSession } from "@/lib/session";
import { listRoles, listUsers } from "@/lib/users";

// Same hover lift as the New user button on the Users tab.
const lift = "transition-[color,background-color,box-shadow] duration-150 hover:shadow-md hover:shadow-black/30";

// Paged in the browser: listRoles loads every role for the dropdowns too.
const PAGE_SIZE = 10;

// The same key as the role dropdowns on the user pages, so a change here reaches them.
const rolesQuery = { queryKey: ["roles"], queryFn: listRoles, staleTime: 5 * 60_000 };

// The Roles tab's cards. They share the panel's query, so no extra request is made.
export function RoleStats() {
  const { isAdmin } = useSession();
  const { data, isPending, isError } = useQuery({ ...rolesQuery, enabled: isAdmin });
  // The panel below already shows the error or the not-allowed message.
  if (!isAdmin || isError) return null;
  if (isPending) return <KpiSkeleton />;
  const system = data.filter((r) => isBuiltIn(r.name)).length;
  // Distinct by id: admin holds every permission, so a plain sum would count most of them twice.
  const permissions = new Set(data.flatMap((r) => r.permissions?.map((p) => p.id) ?? [])).size;
  return (
    <section aria-label="Statistics" className="grid grid-cols-2 gap-4 lg:grid-cols-4 lg:gap-5">
      <StatCard label="Total Roles" value={data.length} icon={Shield} tone="neutral" />
      <StatCard label="Custom Roles" value={data.length - system} icon={ShieldPlus} tone="green" />
      <StatCard label="System Roles" value={system} icon={Lock} tone="sky" />
      <StatCard label="Total Permissions" value={permissions} icon={KeyRound} tone="amber" />
    </section>
  );
}

// The Roles tab of /admin (D-A21): custom roles, then the read-only system roles, each in its own card under an h3.
export function RolesPanel() {
  const { isAdmin, can } = useSession();
  const writable = can("roles:write");
  const [deleting, setDeleting] = useState<RoleDetail | null>(null);
  const [viewing, setViewing] = useState<RoleDetail | null>(null);
  const [page, setPage] = useState(1);
  const opener = useRef<HTMLElement | null>(null);
  const { data, error, isPending, refetch } = useQuery({ ...rolesQuery, enabled: isAdmin });

  let body: React.ReactNode;
  if (!isAdmin || (error instanceof ApiError && error.status === 403)) {
    body = <p className="text-sm text-muted-foreground">You are not allowed to manage roles.</p>;
  } else if (error) {
    body = <ErrorRetry message={error.message} onRetry={() => refetch()} />;
  } else if (isPending) {
    body = <TableSkeleton columns={5} rows={4} />;
  } else {
    const roleTable = (roles: RoleDetail[], heading: string) => (
      <Table aria-labelledby={heading}>
        <TableHeader>
          <TableRow>
            <TableHead>Name</TableHead>
            <TableHead>Description</TableHead>
            <TableHead className="text-right">Users</TableHead>
            <TableHead className="text-right">Permissions</TableHead>
            {writable && (
              <TableHead>
                <span className="sr-only">Actions</span>
              </TableHead>
            )}
          </TableRow>
        </TableHeader>
        <TableBody>
          {roles.map((r) => {
            const builtIn = isBuiltIn(r.name);
            return (
              <TableRow key={r.id}>
                <TableCell>
                  <span className="font-medium">{r.name}</span>
                  {builtIn && (
                    <Badge variant="info" title="System roles cannot be edited or deleted" className="ml-2">
                      <Lock aria-hidden="true" />
                      System
                    </Badge>
                  )}
                </TableCell>
                <TableCell className="text-muted-foreground">{r.description || "—"}</TableCell>
                <TableCell className="text-right font-mono tabular-nums">
                  {r.user_count > 0 ? (
                    <Button
                      variant="link"
                      size="sm"
                      className="p-0 font-mono tabular-nums"
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
                <TableCell className="text-right font-mono tabular-nums">{r.permissions?.length ?? 0}</TableCell>
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
    );
    // Only custom roles are paged; the system roles are a fixed three.
    const custom = pageOf(data.filter((r) => !isBuiltIn(r.name)), page, PAGE_SIZE);
    const system = data.filter((r) => isBuiltIn(r.name));
    // Custom roles first: they are the ones an admin configures (D-A23).
    body = (
      <>
        <Card className={panelCard}>
          <div className="border-b p-5 sm:p-6">
            <h3 id="custom-roles" className="text-base font-medium">
              Custom roles
            </h3>
            <p className="mt-1 text-sm text-muted-foreground">Roles you create, each with its own set of permissions.</p>
          </div>
          <div className="min-w-0 p-5 sm:p-6">
            {custom.rows.length > 0 ? (
              <>
                {roleTable(custom.rows, "custom-roles")}
                <Pagination page={custom.page} totalPages={custom.totalPages} onPageChange={setPage} />
              </>
            ) : (
              <p className="text-sm text-muted-foreground">No custom roles yet.</p>
            )}
          </div>
        </Card>
        <Card className={panelCard}>
          <div className="border-b p-5 sm:p-6">
            <h3 id="system-roles" className="text-base font-medium">
              System roles
            </h3>
            <p className="mt-1 text-sm text-muted-foreground">
              Built into the app. They can be viewed but not edited or deleted.
            </p>
          </div>
          <div className="min-w-0 p-5 sm:p-6">{roleTable(system, "system-roles")}</div>
        </Card>
        <DeleteRoleDialog role={deleting} roles={data} opener={opener} onClose={() => setDeleting(null)} />
        <RoleUsersDialog role={viewing} opener={opener} onClose={() => setViewing(null)} />
      </>
    );
  }

  return (
    <div className="flex min-w-0 flex-col gap-6">
      <header className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h2 className="text-2xl font-semibold tracking-tight">Roles</h2>
          <p className="mt-2 text-sm text-muted-foreground">Control what each role can see and do.</p>
        </div>
        {isAdmin && writable && (
          <Button asChild className={`min-h-11 px-4 ${lift}`}>
            <Link href="/admin/roles/new"><Plus aria-hidden="true" />New role</Link>
          </Button>
        )}
      </header>
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
            {pending ? <><Loader2 aria-hidden="true" className="motion-safe:animate-spin" />Deleting…</> : count > 0 ? "Move users and delete" : "Delete role"}
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
    queryFn: async () =>
      (await Promise.all([listUsers("", "", role!.id), listUsers("", "deleted", role!.id)])).flatMap((p) => p.data),
    enabled: role !== null,
  });

  let body: React.ReactNode;
  if (error) {
    body = <ErrorRetry message={error.message} onRetry={() => refetch()} />;
  } else if (isPending) {
    body = <TableSkeleton columns={2} rows={3} />;
  } else if (data.length === 0) {
    body = <EmptyState title="No accounts hold this role" description="Users assigned to this role will appear here." />;
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
