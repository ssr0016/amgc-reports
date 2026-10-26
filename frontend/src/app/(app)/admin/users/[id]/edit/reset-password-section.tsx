"use client";

import { useState } from "react";
import { toast } from "sonner";
import { AuthField } from "@/components/auth-field";
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
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { toFormError, useCountdown, type FormError } from "@/lib/auth-form";
import { useFocusFirstError } from "@/lib/form-focus";
import { generatePassword } from "@/lib/password";
import { useSession } from "@/lib/session";
import { resetUserPassword, type User } from "@/lib/users";

const LABELS = { new_password: "New password", confirm_password: "Confirm password" } as const;

// Sets another user's password (D-A12). The password lives only in this component's state while
// the dialog is open: after a reset it stays visible once, so it can be copied, and every way of
// closing the dialog clears it. It is never cached, logged or put in a URL.
export function ResetPasswordSection({ user }: { user: Pick<User, "id" | "name"> }) {
  const { user: me } = useSession();
  const self = me?.user_id === user.id;
  const [open, setOpen] = useState(false);
  const [newPw, setNewPw] = useState("");
  const [confirm, setConfirm] = useState("");
  const [generated, setGenerated] = useState(0); // remounts the two fields so they start revealed
  const [done, setDone] = useState<string | null>(null);
  const [error, setError] = useState<FormError | null>(null);
  const [pending, setPending] = useState(false);
  const [copyFailed, setCopyFailed] = useState(false);
  const [wait, setWait] = useCountdown();
  const [formRef, focusError] = useFocusFirstError<HTMLFormElement>();

  function close() {
    setOpen(false);
    setNewPw("");
    setConfirm("");
    setGenerated(0);
    setDone(null);
    setError(null);
    setCopyFailed(false);
    setWait(0);
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    // Empty and "same twice" are checked here, so a typo spends no request; the server owns the password rules.
    const missing: Record<string, string> = {};
    if (!newPw) missing.new_password = "is required";
    if (!confirm) missing.confirm_password = "is required";
    else if (confirm !== newPw) missing.confirm_password = "does not match";
    if (Object.keys(missing).length > 0) {
      setError({ message: "", fields: missing });
      focusError();
      return;
    }

    setError(null);
    setCopyFailed(false);
    setPending(true);
    try {
      await resetUserPassword(user.id, newPw, confirm);
      setDone(newPw);
      setNewPw("");
      setConfirm("");
      toast.success(`Password reset for ${user.name}`);
    } catch (err) {
      const failure = toFormError(err, { 404: "User not found." });
      setError(failure);
      setWait(failure.retryAfter ?? 0);
      focusError();
    } finally {
      setPending(false);
    }
  }

  function generate() {
    const password = generatePassword();
    setNewPw(password);
    setConfirm(password);
    setGenerated((n) => n + 1);
    setError(null);
  }

  async function copy(text: string) {
    try {
      await navigator.clipboard.writeText(text);
      setCopyFailed(false);
      toast.success("Copied");
    } catch {
      setCopyFailed(true); // refused, or no clipboard on an insecure origin: the password stays readable
    }
  }

  const field = (name: keyof typeof LABELS) => {
    const msg = error?.fields[name];
    return msg ? `${LABELS[name]} ${msg}` : undefined;
  };

  return (
    <fieldset className="flex max-w-xl flex-col gap-3 border-t pt-4">
      <legend className="text-lg font-medium">Reset password</legend>
      <p className="text-sm text-muted-foreground">Sets a new password for {user.name} and signs them out everywhere.</p>
      {self && (
        <p id="reset-password-note" className="text-sm text-muted-foreground">
          You cannot reset your own password. Use the Change Password page instead.
        </p>
      )}
      <div>
        <Button
          type="button"
          variant="outline"
          disabled={self}
          aria-describedby={self ? "reset-password-note" : undefined}
          onClick={() => setOpen(true)}
        >
          Reset password
        </Button>
      </div>

      <AlertDialog open={open} onOpenChange={(next) => !next && !pending && close()}>
        <AlertDialogContent>
          {done === null ? (
            <form ref={formRef} onSubmit={submit} noValidate className="flex flex-col gap-4">
              <AlertDialogHeader>
                <AlertDialogTitle>Reset password for {user.name}</AlertDialogTitle>
                <AlertDialogDescription>
                  {user.name} will be signed out everywhere and must sign in with the new password. Give it to them
                  yourself: it is not stored, and not shown again after this.
                </AlertDialogDescription>
              </AlertDialogHeader>
              <AuthField
                key={`new-${generated}`}
                name="new_password"
                label={LABELS.new_password}
                type="password"
                autoComplete="new-password"
                defaultShown={generated > 0}
                disabled={pending}
                value={newPw}
                onChange={(e) => setNewPw(e.target.value)}
                error={field("new_password")}
              />
              <AuthField
                key={`confirm-${generated}`}
                name="confirm_password"
                label={LABELS.confirm_password}
                type="password"
                autoComplete="new-password"
                defaultShown={generated > 0}
                disabled={pending}
                value={confirm}
                onChange={(e) => setConfirm(e.target.value)}
                error={field("confirm_password")}
              />
              <div className="flex flex-wrap gap-2">
                <Button type="button" variant="outline" onClick={generate} disabled={pending}>
                  Generate random password
                </Button>
                <Button type="button" variant="outline" onClick={() => copy(newPw)} disabled={pending || !newPw}>
                  Copy password
                </Button>
              </div>
              {copyFailed && (
                <p role="status" className="text-sm text-destructive">
                  Could not copy. Select the password and copy it yourself.
                </p>
              )}
              {error?.message && (
                <p role="alert" className="text-sm text-destructive">
                  {error.message}
                  {wait > 0 && ` Try again in ${wait}s.`}
                </p>
              )}
              <AlertDialogFooter>
                <AlertDialogCancel disabled={pending}>Cancel</AlertDialogCancel>
                {/* A plain submit Button, not AlertDialogAction: that one closes the dialog on click, and a failed reset must be shown. */}
                <Button type="submit" disabled={pending || wait > 0}>
                  {pending ? "Resetting…" : "Reset password"}
                </Button>
              </AlertDialogFooter>
            </form>
          ) : (
            <div className="flex flex-col gap-4">
              <AlertDialogHeader>
                <AlertDialogTitle>Password reset</AlertDialogTitle>
                <AlertDialogDescription>
                  {user.name} is signed out everywhere. This password will not be shown again. Copy it now and give it to them.
                </AlertDialogDescription>
              </AlertDialogHeader>
              <div className="flex flex-col gap-2">
                <Label htmlFor="reset-done-password">Password</Label>
                <Input id="reset-done-password" readOnly value={done} onFocus={(e) => e.currentTarget.select()} />
              </div>
              {copyFailed && (
                <p role="status" className="text-sm text-destructive">
                  Could not copy. Select the password and copy it yourself.
                </p>
              )}
              <AlertDialogFooter>
                <Button type="button" variant="outline" onClick={() => copy(done)}>
                  Copy password
                </Button>
                <AlertDialogCancel>Close</AlertDialogCancel>
              </AlertDialogFooter>
            </div>
          )}
        </AlertDialogContent>
      </AlertDialog>
    </fieldset>
  );
}
