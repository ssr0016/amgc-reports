"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { toast } from "sonner";
import { UserForm } from "@/components/user-form";
import { ApiError } from "@/lib/api";
import { useSession } from "@/lib/session";
import { getUser, updateUser } from "@/lib/users";
import { ResetPasswordSection } from "./reset-password-section";
import { RoleSection } from "./role-section";
import { StatusSection } from "./status-section";
import { refreshAdminViews } from "@/lib/query";
import { TableSkeleton } from "@/components/loading-skeletons";

export function EditUser({ id }: { id: number }) {
  const router = useRouter();
  const queryClient = useQueryClient();
  const { isAdmin } = useSession();
  const userId = id;
  const valid = Number.isInteger(userId) && userId > 0;

  // gcTime 0: PUT replaces name, area and church, so the form must start from a fresh load.
  const { data, error, isPending } = useQuery({
    queryKey: ["user", userId],
    queryFn: () => getUser(userId),
    enabled: valid && isAdmin,
    gcTime: 0,
  });

  let body: React.ReactNode;
  if (!isAdmin) {
    body = <p className="text-sm text-muted-foreground">You are not allowed to edit users.</p>;
  } else if (!valid || (error instanceof ApiError && error.status === 404)) {
    body = <p className="text-sm text-muted-foreground">User not found.</p>;
  } else if (error) {
    body = (
      <p role="alert" className="text-sm text-destructive">
        {error.message}
      </p>
    );
  } else if (isPending) {
    body = <TableSkeleton columns={2} rows={5} />;
  } else {
    body = (
      <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_20rem] lg:items-start">
        <UserForm
          mode="edit"
          initial={{
            email: data.email,
            name: data.name,
            password: "",
            confirm_password: "",
            role_id: "",
            area_assignment: data.area_assignment ?? "",
            church_name: data.church_name ?? "",
          }}
          submitLabel="Update User"
          onSubmit={async (v) => {
            await updateUser(userId, {
              name: v.name.trim(),
              area_assignment: v.area_assignment.trim(),
              church_name: v.church_name.trim(),
            });
            toast.success(`Saved ${v.name.trim()}`);
            await Promise.all([queryClient.invalidateQueries({ queryKey: ["users"] }), refreshAdminViews(queryClient)]);
            router.replace("/admin?tab=users");
          }}
          onCancel={() => router.push("/admin?tab=users")}
        />
        {/* Each action is its own card with its own request, apart from the profile Save. */}
        <section aria-labelledby="account-actions" className="flex flex-col gap-4">
          <h2 id="account-actions" className="text-xs font-medium tracking-wide text-muted-foreground uppercase">
            Account actions
          </h2>
          <StatusSection user={data} />
          <RoleSection user={data} />
          <ResetPasswordSection user={data} />
        </section>
      </div>
    );
  }

  return (
    <main id="main-content" tabIndex={-1} className="mx-auto flex w-full max-w-5xl flex-1 flex-col gap-6 p-4 focus:outline-none">
      <h1 className="text-2xl font-medium tracking-tight">Edit user</h1>
      {body}
    </main>
  );
}
