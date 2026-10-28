"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { toast } from "sonner";
import { ErrorRetry } from "@/components/error-retry";
import { FormSkeleton } from "@/components/loading-skeletons";
import { dashboardTheme } from "@/components/dashboard-theme";
import { RoleForm } from "@/components/role-form";
import { createRole, listPermissions } from "@/lib/roles";
import { useSession } from "@/lib/session";
import { PageHeader, PageShell } from "@/components/page-shell";

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
    body = <FormSkeleton fields={3} />;
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
    <PageShell className={`${dashboardTheme}`}>
      <PageHeader title="New role" description="Pick a name and the permissions this role grants." />
      {body}
    </PageShell>
  );
}
