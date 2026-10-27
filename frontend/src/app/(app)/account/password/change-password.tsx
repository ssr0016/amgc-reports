"use client";

import { useState } from "react";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";
import { AuthField } from "@/components/auth-field";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { changePassword } from "@/lib/auth";
import { toFormError, useCountdown, type FormError } from "@/lib/auth-form";
import { useFocusFirstError } from "@/lib/form-focus";

const LABELS = { current_password: "Current password", new_password: "New password", confirm_password: "Confirm password" } as const;

export function ChangePassword() {
  const [error, setError] = useState<FormError | null>(null);
  const [pending, setPending] = useState(false);
  const [wait, setWait] = useCountdown();
  const [formRef, focusError] = useFocusFirstError<HTMLFormElement>();

  async function onSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const form = e.currentTarget;
    const data = new FormData(form);
    const current = String(data.get("current_password"));
    const next = String(data.get("new_password"));
    const confirm = String(data.get("confirm_password"));

    // Empty and "same twice" are checked here, so a typo spends no request; the server owns the password rules.
    const missing: Record<string, string> = {};
    if (!current) missing.current_password = "is required";
    if (!next) missing.new_password = "is required";
    if (!confirm) missing.confirm_password = "is required";
    else if (confirm !== next) missing.confirm_password = "does not match";
    if (Object.keys(missing).length > 0) {
      setError({ message: "", fields: missing });
      focusError();
      return;
    }

    setError(null);
    setPending(true);
    try {
      await changePassword(current, next, confirm);
      form.reset();
      toast.success("Password changed");
    } catch (err) {
      const formError = toFormError(err, { 403: "This account is locked. Try again later." });
      setError(formError);
      setWait(formError.retryAfter ?? 0);
      focusError();
    } finally {
      setPending(false);
    }
  }

  const field = (name: keyof typeof LABELS) => {
    const msg = error?.fields[name];
    return msg ? `${LABELS[name]} ${msg}` : undefined;
  };

  return (
    <main id="main-content" tabIndex={-1} className="mx-auto flex w-full max-w-xl flex-1 flex-col gap-6 p-4 sm:p-6 focus:outline-none">
      <h1 className="text-2xl font-medium tracking-tight">Change Password</h1>
      <Card className="p-4 sm:p-6">
      <form ref={formRef} onSubmit={onSubmit} noValidate className="flex flex-col gap-4">
        <AuthField
          name="current_password"
          label={LABELS.current_password}
          type="password"
          autoComplete="current-password"
          error={field("current_password")}
        />
        <AuthField
          name="new_password"
          label={LABELS.new_password}
          type="password"
          autoComplete="new-password"
          error={field("new_password")}
        />
        <AuthField
          name="confirm_password"
          label={LABELS.confirm_password}
          type="password"
          autoComplete="new-password"
          error={field("confirm_password")}
        />
        {error?.message && (
          <p role="alert" className="text-sm text-destructive">
            {error.message}
            {wait > 0 && ` Try again in ${wait}s.`}
          </p>
        )}
        <Button type="submit" disabled={pending || wait > 0}>
          {pending ? <><Loader2 aria-hidden="true" className="motion-safe:animate-spin" />Changing…</> : "Change Password"}
        </Button>
      </form>
      </Card>
    </main>
  );
}
