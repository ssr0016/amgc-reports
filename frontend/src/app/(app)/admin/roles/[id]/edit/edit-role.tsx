"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { toast } from "sonner";
import { ErrorRetry } from "@/components/error-retry";
import { TableSkeleton } from "@/components/loading-skeletons";
import { dashboardTheme } from "@/components/dashboard-theme";
import { RoleForm } from "@/components/role-form";
import { ApiError } from "@/lib/api";
import { getRole, isBuiltIn, listPermissions, updateRole } from "@/lib/roles";
import { useSession } from "@/lib/session";

export function EditRole({ id }: { id: number }) {
  const router = useRouter();
  const queryClient = useQueryClient();
  const { isAdmin, can } = useSession();
  const allowed = isAdmin && can("roles:write");
  const roleId = id;
  const valid = Number.isInteger(roleId) && roleId > 0;

  // gcTime 0: PUT replaces the whole role, so the form must start from a fresh load.
  const role = useQuery({ queryKey: ["role", roleId], queryFn: () => getRole(roleId), enabled: valid && allowed, gcTime: 0 });
  const perms = useQuery({ queryKey: ["permissions"], queryFn: listPermissions, enabled: allowed, staleTime: 5 * 60_000 });
  const error = role.error ?? perms.error;

  let body: React.ReactNode;
  if (!allowed) {
    body = <p className="text-sm text-muted-foreground">You are not allowed to manage roles.</p>;
  } else if (!valid || (role.error instanceof ApiError && role.error.status === 404)) {
    body = <p className="text-sm text-muted-foreground">Role not found.</p>;
  } else if (error) {
    body = <ErrorRetry message={error.message} onRetry={() => (role.error ? role.refetch() : perms.refetch())} />;
  } else if (!role.data || !perms.data) {
    body = <TableSkeleton columns={2} rows={4} />;
  } else {
    body = (
      <RoleForm
        initial={{
          name: role.data.name,
          description: role.data.description,
          permission_ids: (role.data.permissions ?? []).map((p) => p.id!),
        }}
        permissions={perms.data}
        readOnly={isBuiltIn(role.data.name)}
        submitLabel="Save changes"
        onSubmit={async (v) => {
          const saved = await updateRole(roleId, v);
          toast.success(`Saved role ${saved.name}`);
          await queryClient.invalidateQueries({ queryKey: ["roles"] });
          router.replace("/admin?tab=roles");
        }}
        onCancel={() => router.push("/admin?tab=roles")}
      />
    );
  }

  return (
    <main id="main-content" tabIndex={-1} className={`${dashboardTheme} account-surface mx-auto flex w-full max-w-5xl flex-1 flex-col gap-6 p-4 sm:p-6 focus:outline-none`}>
      <header>
        <h1 className="text-2xl font-semibold tracking-tight">Edit role</h1>
        <p className="mt-2 text-sm text-muted-foreground">Saving replaces the name, description and every permission.</p>
      </header>
      {body}
    </main>
  );
}
