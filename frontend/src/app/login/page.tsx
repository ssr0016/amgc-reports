"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { ApiError } from "@/lib/api";
import { login } from "@/lib/auth";

export default function LoginPage() {
  const router = useRouter();
  const [error, setError] = useState("");
  const [pending, setPending] = useState(false);

  async function onSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const form = new FormData(e.currentTarget);
    setError("");
    setPending(true);
    try {
      await login(String(form.get("email")), String(form.get("password")));
      router.replace("/");
    } catch (err) {
      if (!(err instanceof ApiError)) throw err;
      const fields = err.details ? Object.values(err.details).join(" ") : "";
      const wait = err.status === 429 && err.retryAfter ? ` Try again in ${err.retryAfter}s.` : "";
      setError((fields || err.message) + wait);
      setPending(false);
    }
  }

  return (
    <main className="flex flex-1 items-center justify-center p-4">
      <form onSubmit={onSubmit} className="flex w-full max-w-sm flex-col gap-4">
        <h1 className="text-2xl font-semibold tracking-tight">Sign in</h1>
        <label className="flex flex-col gap-1 text-sm">
          Email
          <input
            name="email"
            type="email"
            autoComplete="email"
            required
            className="rounded border border-zinc-400 px-3 py-2 text-base"
          />
        </label>
        <label className="flex flex-col gap-1 text-sm">
          Password
          <input
            name="password"
            type="password"
            autoComplete="current-password"
            required
            className="rounded border border-zinc-400 px-3 py-2 text-base"
          />
        </label>
        {error && (
          <p role="alert" className="text-sm text-red-600">
            {error}
          </p>
        )}
        <button
          type="submit"
          disabled={pending}
          className="rounded bg-foreground px-3 py-2 text-background disabled:opacity-50"
        >
          {pending ? "Signing in…" : "Sign in"}
        </button>
      </form>
    </main>
  );
}
