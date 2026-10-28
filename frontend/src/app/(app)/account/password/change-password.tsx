"use client";

import { useState } from "react";
import Link from "next/link";
import { ArrowLeft, KeyRound, Loader2, Lock } from "lucide-react";
import { toast } from "sonner";
import { AuthField } from "@/components/auth-field";
import { dashboardTheme } from "@/components/dashboard-theme";
import { PanelSection, panelCard } from "@/components/panel-section";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { changePassword } from "@/lib/auth";
import { toFormError, useCountdown, type FormError } from "@/lib/auth-form";
import { useFocusFirstError } from "@/lib/form-focus";
import { PageHeader, PageShell } from "@/components/page-shell";

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
    <PageShell className={`${dashboardTheme} max-w-xl`}>
      <PageHeader title="Change password" description="Use a strong password you don’t use anywhere else." />
      <form ref={formRef} onSubmit={onSubmit} noValidate>
        <Card className={panelCard}>
          <PanelSection title="Password" icon={Lock} description="Enter your current password, then choose a new one.">
            <AuthField
              name="current_password"
              label={LABELS.current_password}
              type="password"
              autoComplete="current-password"
              icon={Lock}
              error={field("current_password")}
            />
            <AuthField
              name="new_password"
              label={LABELS.new_password}
              type="password"
              autoComplete="new-password"
              icon={KeyRound}
              error={field("new_password")}
            />
            <AuthField
              name="confirm_password"
              label={LABELS.confirm_password}
              type="password"
              autoComplete="new-password"
              icon={KeyRound}
              error={field("confirm_password")}
            />
            {error?.message && (
              <p role="alert" className="text-sm text-destructive">
                {error.message}
                {wait > 0 && ` Try again in ${wait}s.`}
              </p>
            )}
          </PanelSection>
          {/* Same footer as UserForm: primary action on top on phones, on the right on wider screens. */}
          <div className="flex flex-col-reverse gap-2 border-t bg-muted/30 p-5 sm:flex-row sm:justify-between sm:p-6">
            <Button asChild variant="outline" className="min-h-11 px-4">
              <Link href="/account/profile"><ArrowLeft aria-hidden="true" />Back to Profile</Link>
            </Button>
            <Button type="submit" className="min-h-11 px-4" disabled={pending || wait > 0}>
              {pending ? <><Loader2 aria-hidden="true" className="motion-safe:animate-spin" />Changing…</> : "Change Password"}
            </Button>
          </div>
        </Card>
      </form>
    </PageShell>
  );
}
