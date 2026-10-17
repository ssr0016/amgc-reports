"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useParams, useRouter } from "next/navigation";
import { UserForm } from "@/components/user-form";
import { ApiError } from "@/lib/api";
import { useSession } from "@/lib/session";
import { getUser, updateUser } from "@/lib/users";
import { StatusSection } from "./status-section";

export function EditUser() {
  const router = useRouter();
  const queryClient = useQueryClient();
  const { isAdmin } = useSession();
  const userId = Number(useParams<{ id: string }>().id);
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
    body = <p role="status" className="text-sm text-muted-foreground">Loading…</p>;
  } else {
    body = (
      <>
        <UserForm
          mode="edit"
          initial={{
            email: data.email,
            name: data.name,
            password: "",
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
            await queryClient.invalidateQueries({ queryKey: ["users"] });
            router.replace("/admin?tab=users");
          }}
          onCancel={() => router.push("/admin?tab=users")}
        />
        <StatusSection user={data} />
      </>
    );
  }

  return (
    <main id="main-content" tabIndex={-1} className="mx-auto flex w-full max-w-5xl flex-1 flex-col gap-6 p-4 focus:outline-none">
      <h1 className="text-2xl font-semibold tracking-tight">Edit user</h1>
      {body}
    </main>
  );
}
