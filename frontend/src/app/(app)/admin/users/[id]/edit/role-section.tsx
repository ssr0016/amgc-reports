"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { NativeSelect } from "@/components/ui/native-select";
import { ApiError } from "@/lib/api";
import { refreshAdminViews } from "@/lib/query";
import { changeUserRole } from "@/lib/roles";
import { useSession } from "@/lib/session";
import { listRoles, type User } from "@/lib/users";

// Its own request, like the Status section: the profile PUT never carries the role (D-A21).
export function RoleSection({ user }: { user: Pick<User, "id" | "name" | "role_id"> }) {
  const queryClient = useQueryClient();
  const { user: me } = useSession();
  const self = me?.user_id === user.id;
  const roles = useQuery({ queryKey: ["roles"], queryFn: listRoles, enabled: !self, staleTime: 5 * 60_000 });
  const [current, setCurrent] = useState(user.role_id);
  const [selected, setSelected] = useState(String(user.role_id));
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");

  async function save() {
    setPending(true);
    setError("");
    try {
      const saved = await changeUserRole(user.id, Number(selected));
      setCurrent(saved.role_id);
      const name = roles.data?.find((r) => r.id === saved.role_id)?.name ?? "the new role";
      toast.success(`${user.name} now has the ${name} role`);
      // The role list counts users, and the Tracker counts only the user role.
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["users"] }),
        queryClient.invalidateQueries({ queryKey: ["user", user.id] }),
        queryClient.invalidateQueries({ queryKey: ["roles"] }),
        refreshAdminViews(queryClient),
      ]);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not change the role. Try again.");
    } finally {
      setPending(false);
    }
  }

  return (
    <fieldset className="flex max-w-xl flex-col gap-3 border-t pt-4">
      <legend className="text-lg font-medium">Role</legend>
      {self ? (
        <p className="text-sm text-muted-foreground">You cannot change your own role.</p>
      ) : roles.error ? (
        <p role="alert" className="text-sm text-destructive">
          Could not load the roles. Reload the page to try again.
        </p>
      ) : (
        <>
          <Label htmlFor="user-role" className="sr-only">
            Role
          </Label>
          <NativeSelect
            id="user-role"
            value={selected}
            disabled={pending || !roles.data}
            onChange={(e) => setSelected(e.target.value)}
          >
            {roles.data ? (
              roles.data.map((r) => (
                <option key={r.id} value={r.id}>
                  {r.name}
                </option>
              ))
            ) : (
              <option value={selected}>Loading…</option>
            )}
          </NativeSelect>
          {error && (
            <p role="alert" className="text-sm text-destructive">
              {error}
            </p>
          )}
          <div>
            <Button type="button" onClick={save} disabled={pending || !roles.data || Number(selected) === current}>
              {pending ? "Saving…" : "Update role"}
            </Button>
          </div>
        </>
      )}
    </fieldset>
  );
}
