"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { UserForm } from "@/components/user-form";
import { useSession } from "@/lib/session";
import { createUser, listRoles } from "@/lib/users";

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
    body = <p className="text-sm text-muted-foreground">Loading…</p>;
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
            role_id: v.role_id ? Number(v.role_id) : undefined,
            area_assignment: v.area_assignment.trim(),
            church_name: v.church_name.trim(),
          });
          await queryClient.invalidateQueries({ queryKey: ["users"] });
          router.replace("/admin/users");
        }}
        onCancel={() => router.push("/admin/users")}
      />
    );
  }

  return (
    <main className="mx-auto flex w-full max-w-5xl flex-1 flex-col gap-6 p-4">
      <h1 className="text-2xl font-semibold tracking-tight">New user</h1>
      {body}
    </main>
  );
}
