"use client";

import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
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
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { NativeSelect } from "@/components/ui/native-select";
import { ApiError } from "@/lib/api";
import { useSession } from "@/lib/session";
import { STATUS_LABEL, statusLabel, USER_STATUSES } from "@/lib/user-status";
import { changeUserStatus, type User } from "@/lib/users";
import { refreshAdminViews } from "@/lib/query";

// Its own request, apart from the profile form: the profile PUT never carries the status.
export function StatusSection({
  user,
}: {
  user: Pick<User, "id" | "name" | "status">;
}) {
  const queryClient = useQueryClient();
  const { user: me } = useSession();
  const self = me?.user_id === user.id;
  const [selected, setSelected] = useState(user.status);
  const [confirming, setConfirming] = useState(false);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");

  async function apply() {
    setPending(true);
    setError("");
    try {
      await changeUserStatus(user.id, selected);
      toast.success(`${user.name} is now ${statusLabel(selected)}`);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["users"] }),
        queryClient.invalidateQueries({ queryKey: ["reports"] }),
        queryClient.invalidateQueries({ queryKey: ["user", user.id] }),
        refreshAdminViews(queryClient),
      ]);
    } catch (err) {
      setError(
        err instanceof ApiError
          ? err.message
          : "Could not change the status. Try again.",
      );
    } finally {
      setPending(false);
      setConfirming(false);
    }
  }

  // Leaving Active signs the user out everywhere, so it asks first; going back to Active does not.
  const submit = () => (selected === "active" ? apply() : setConfirming(true));

  return (
    <fieldset className="flex max-w-xl flex-col gap-3 border-t pt-4">
      <legend className="text-lg font-semibold">Status</legend>
      <Label htmlFor="user-status" className="sr-only">
        Status
      </Label>
      <NativeSelect
        id="user-status"
        value={selected}
        disabled={self || pending}
        aria-describedby={self ? "user-status-note" : undefined}
        onChange={(e) => setSelected(e.target.value)}
      >
        {USER_STATUSES.map((s) => (
          <option key={s} value={s}>
            {STATUS_LABEL[s]}
          </option>
        ))}
      </NativeSelect>
      {self && (
        <p id="user-status-note" className="text-sm text-muted-foreground">
          You cannot change your own status.
        </p>
      )}
      {error && (
        <p role="alert" className="text-sm text-destructive">
          {error}
        </p>
      )}
      <div>
        <Button
          type="button"
          onClick={submit}
          disabled={self || pending || selected === user.status}
        >
          Update status
        </Button>
      </div>

      <AlertDialog
        open={confirming}
        onOpenChange={(open) => !open && !pending && setConfirming(false)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              Set {user.name} to {statusLabel(selected)}?
            </AlertDialogTitle>
            <AlertDialogDescription>
              {user.name} will be signed out everywhere and cannot sign in until
              the status is set back to Active.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={pending}>Cancel</AlertDialogCancel>
            {/* A plain Button, not AlertDialogAction: that one closes the dialog on click, and a failed change must be shown. */}
            <Button onClick={apply} disabled={pending}>
              {pending ? "Saving…" : "Change status"}
            </Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </fieldset>
  );
}
