"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { toast } from "sonner";
import { ErrorRetry } from "@/components/error-retry";
import { Loading } from "@/components/loading";
import { dashboardTheme } from "@/components/dashboard-theme";
import { RoleForm } from "@/components/role-form";
import { createRole, listPermissions } from "@/lib/roles";
import { useSession } from "@/lib/session";

export function NewRole() {
  const router = useRouter();
  const queryClient = useQueryClient();
  const { isAdmin, can } = useSession();
  const allowed = isAdmin && can("roles:write");
  const perms = useQuery({ queryKey: ["permissions"], queryFn: listPermissions, enabled: allowed, staleTime: 5 * 60_000 });

  let body: React.ReactNode;
  if (!allowed) {
    body = <p className="text-sm text-muted-foreground">You are not allowed to manage roles.</p>;
  } else if (perms.error) {
    body = <ErrorRetry message={perms.error.message} onRetry={() => perms.refetch()} />;
  } else if (perms.isPending) {
    body = <Loading />;
  } else {
    body = (
      <RoleForm
        permissions={perms.data}
        submitLabel="Create role"
        onSubmit={async (v) => {
          const role = await createRole(v);
          toast.success(`Created role ${role.name}`);
          await queryClient.invalidateQueries({ queryKey: ["roles"] });
          router.replace("/admin?tab=roles");
        }}
        onCancel={() => router.push("/admin?tab=roles")}
      />
    );
  }

  return (
    <main id="main-content" tabIndex={-1} className={`${dashboardTheme} account-surface mx-auto flex w-full max-w-5xl flex-1 flex-col gap-6 p-4 sm:p-6 focus:outline-none`}>
      <h1 className="text-2xl font-semibold tracking-tight">New role</h1>
      {body}
    </main>
  );
}
