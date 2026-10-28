"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { toast } from "sonner";
import { UserForm } from "@/components/user-form";
import { useSession } from "@/lib/session";
import { createUser, listRoles } from "@/lib/users";
import { refreshAdminViews } from "@/lib/query";
import { FormSkeleton } from "@/components/loading-skeletons";
import { PageHeader, PageShell } from "@/components/page-shell";

export function NewUser() {
  const router = useRouter();
  const queryClient = useQueryClient();
  const { isAdmin } = useSession();
  const roles = useQuery({ queryKey: ["roles"], queryFn: listRoles, enabled: isAdmin, staleTime: 5 * 60_000 });

  let body: React.ReactNode;
  if (!isAdmin) {
    body = <p className="text-sm text-muted-foreground">You are not allowed to create users.</p>;
  } else if (roles.isPending) {
    // UserForm reads the roles once, on mount, so it waits for them. If they failed it still
    // renders, without the select, and the server picks the default role.
    body = <FormSkeleton fields={5} />;
  } else {
    body = (
      <UserForm
        mode="create"
        roles={roles.data ?? []}
        submitLabel="Create User"
        onSubmit={async (v) => {
          await createUser({
            email: v.email.trim(),
            name: v.name.trim(),
            password: v.password,
            confirm_password: v.confirm_password,
            role_id: v.role_id ? Number(v.role_id) : undefined,
            area_assignment: v.area_assignment.trim(),
            church_name: v.church_name.trim(),
          });
          toast.success(`Created user ${v.name.trim()}`);
          await Promise.all([queryClient.invalidateQueries({ queryKey: ["users"] }), refreshAdminViews(queryClient)]);
          router.replace("/admin?tab=users");
        }}
        onCancel={() => router.push("/admin?tab=users")}
      />
    );
  }

  return (
    <PageShell>
      <PageHeader title="New user" />
      {body}
    </PageShell>
  );
}
