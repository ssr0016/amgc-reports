"use client";

import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";
import { dashboardTheme } from "@/components/dashboard-theme";
import { AuthField } from "@/components/auth-field";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { ApiError } from "@/lib/api";
import { safeNext, toFormError, useCountdown, type FormError } from "@/lib/auth-form";
import { useSession } from "@/lib/session";

export default function LoginPage() {
  const router = useRouter();
  const { signIn, status, isAdmin } = useSession();
  const [error, setError] = useState<FormError | null>(null);
  const [pending, setPending] = useState(false);
  const [wait, setWait] = useCountdown();

  // Already signed in (proxy.ts sent a session without its marker cookie here): go back.
  useEffect(() => {
    if (status === "authenticated") {
      router.replace(safeNext(new URLSearchParams(location.search).get("next"), isAdmin ? "/admin" : "/reports"));
    }
  }, [status, isAdmin, router]);

  async function onSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const formEl = e.currentTarget;
    const form = new FormData(formEl);
    setError(null);
    setPending(true);
    try {
      await signIn(String(form.get("email")), String(form.get("password")));
      // cacheComponents keeps this page alive (hidden) after we leave, so reset it here, or the
      // next visit after signing out shows a stuck "Signing in…" and the last password.
      formEl.reset();
      setPending(false);
    } catch (err) {
      // A 403 is the server's own sentence: locked, inactive, under review or closed.
      const formError = toFormError(err, { 401: "Invalid email or password." });
      setError(formError);
      if (err instanceof ApiError && err.status === 401) toast.error("Invalid credentials. Please try again.");
      setWait(formError.retryAfter ?? 0);
      setPending(false);
    }
  }

  return (
    <main className={`${dashboardTheme} flex flex-1 items-center justify-center px-4 py-12 sm:py-20`}>
      <div className="w-full max-w-md">
        <p className="mb-8 text-center text-xl font-medium tracking-tight">AMGCReports</p>
        <Card className="w-full gap-6 rounded-lg border py-6 sm:py-8">
          <CardHeader className="gap-3 px-6 sm:px-8">
            <CardTitle className="text-2xl font-medium tracking-tight">
              <h1>Sign in</h1>
            </CardTitle>
            <CardDescription>Use the account your admin gave you.</CardDescription>
          </CardHeader>
          <CardContent className="px-6 sm:px-8">
            <form onSubmit={onSubmit} className="flex flex-col gap-5">
              <AuthField name="email" label="Email" type="email" autoComplete="email" required error={error?.fields.email} />
              <AuthField
                name="password"
                label="Password"
                type="password"
                autoComplete="current-password"
                required
                error={error?.fields.password}
              />
              {error?.message && (
                <p role="alert" className="text-sm text-destructive">
                  {error.message}
                  {wait > 0 && ` Try again in ${wait}s.`}
                </p>
              )}
              {/* Still "Signing in…" while the redirect away from here runs. */}
              <Button type="submit" className="mt-1 min-h-11 w-full" disabled={pending || wait > 0 || status === "authenticated"} aria-busy={pending}>
                {pending || status === "authenticated" ? <><Loader2 aria-hidden="true" className="motion-safe:animate-spin" />Signing in…</> : "Sign in"}
              </Button>
            </form>
          </CardContent>
        </Card>
      </div>
    </main>
  );
}
