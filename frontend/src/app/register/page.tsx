"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { AuthField } from "@/components/auth-field";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { api, ApiError } from "@/lib/api";
import { toFormError, useCountdown, type FormError } from "@/lib/auth-form";
import { useSession } from "@/lib/session";

export default function RegisterPage() {
  const router = useRouter();
  const { signIn } = useSession();
  const [error, setError] = useState<FormError | null>(null);
  const [pending, setPending] = useState(false);
  const [wait, setWait] = useCountdown();

  async function onSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const form = new FormData(e.currentTarget);
    const email = String(form.get("email"));
    const password = String(form.get("password"));
    setError(null);
    setPending(true);
    try {
      await api("/auth/register", { method: "POST", body: { email, name: String(form.get("name")), password } });
      await signIn(email, password);
      router.replace("/");
    } catch (err) {
      // A taken email belongs on the email field, not in the general message.
      const formError =
        err instanceof ApiError && err.isConflict
          ? { message: "", fields: { email: "This email is already registered." } }
          : toFormError(err);
      setError(formError);
      setWait(formError.retryAfter ?? 0);
      setPending(false);
    }
  }

  return (
    <main className="flex flex-1 items-center justify-center p-4">
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle className="text-xl">Create an account</CardTitle>
          <CardDescription>You will be signed in right after.</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={onSubmit} className="flex flex-col gap-4">
            <AuthField name="name" label="Name" autoComplete="name" required minLength={2} maxLength={100} error={error?.fields.name} />
            <AuthField name="email" label="Email" type="email" autoComplete="email" required error={error?.fields.email} />
            <AuthField
              name="password"
              label="Password"
              type="password"
              autoComplete="new-password"
              required
              hint="Use a long, hard-to-guess password."
              error={error?.fields.password}
            />
            {error?.message && (
              <p role="alert" className="text-sm text-destructive">
                {error.message}
                {wait > 0 && ` Try again in ${wait}s.`}
              </p>
            )}
            <Button type="submit" disabled={pending || wait > 0}>
              {pending ? "Creating account…" : "Register"}
            </Button>
            <p className="text-sm text-muted-foreground">
              Already registered?{" "}
              <Link href="/login" className="underline underline-offset-4">
                Sign in
              </Link>
            </p>
          </form>
        </CardContent>
      </Card>
    </main>
  );
}
