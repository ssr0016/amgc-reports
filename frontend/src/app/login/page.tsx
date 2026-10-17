"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { AuthField } from "@/components/auth-field";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { safeNext, toFormError, useCountdown, type FormError } from "@/lib/auth-form";
import { useSession } from "@/lib/session";

export default function LoginPage() {
  const router = useRouter();
  const { signIn } = useSession();
  const [error, setError] = useState<FormError | null>(null);
  const [pending, setPending] = useState(false);
  const [wait, setWait] = useCountdown();

  async function onSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const form = new FormData(e.currentTarget);
    setError(null);
    setPending(true);
    try {
      await signIn(String(form.get("email")), String(form.get("password")));
      router.replace(safeNext(new URLSearchParams(location.search).get("next")));
    } catch (err) {
      // A 403 is the server's own sentence: locked, inactive, under review or closed.
      const formError = toFormError(err, { 401: "Invalid email or password." });
      setError(formError);
      setWait(formError.retryAfter ?? 0);
      setPending(false);
    }
  }

  return (
    <main className="flex flex-1 items-center justify-center p-4">
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle className="text-xl">
            <h1>Sign in</h1>
          </CardTitle>
          <CardDescription>Use the account your admin gave you.</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={onSubmit} className="flex flex-col gap-4">
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
            <Button type="submit" disabled={pending || wait > 0}>
              {pending ? "Signing in…" : "Sign in"}
            </Button>
          </form>
        </CardContent>
      </Card>
    </main>
  );
}
