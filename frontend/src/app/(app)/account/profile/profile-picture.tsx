"use client";

import { useState } from "react";
import { toast } from "sonner";
import { Avatar } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ApiError } from "@/lib/api";
import { resizeAvatar, uploadAvatar } from "@/lib/avatar";
import { useCountdown } from "@/lib/auth-form";
import { queryClient } from "@/lib/query";
import { useSession } from "@/lib/session";

export function ProfilePicture() {
  const { user, updateAvatar } = useSession();
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const [wait, setWait] = useCountdown();

  async function onSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (pending || wait > 0) return;
    const form = event.currentTarget;
    const file = form.querySelector<HTMLInputElement>('input[type="file"]')?.files?.[0];
    if (!file?.size) {
      setError("Choose an image first.");
      form.querySelector<HTMLInputElement>("input")?.focus();
      return;
    }
    setError("");
    setPending(true);
    try {
      const saved = await uploadAvatar(await resizeAvatar(file));
      updateAvatar(saved.id, saved.avatar_url ?? "");
      void queryClient.invalidateQueries({ queryKey: ["users"] });
      void queryClient.invalidateQueries({ queryKey: ["user", saved.id] });
      form.reset();
      toast.success("Profile picture updated");
    } catch (err) {
      if (err instanceof ApiError) {
        setError(err.status === 413 ? "This image is too large. Choose another file." : err.details?.avatar ?? err.message);
        setWait(err.retryAfter ?? 0);
      } else {
        setError(err instanceof Error ? err.message : "Could not upload this image. Please try again.");
      }
    } finally {
      setPending(false);
    }
  }

  return (
    <main id="main-content" tabIndex={-1} className="mx-auto flex w-full max-w-sm flex-1 flex-col gap-6 p-4 focus:outline-none">
      <h1 className="text-2xl font-semibold tracking-tight">Profile picture</h1>
      <div className="flex items-center gap-3">
        <Avatar name={user?.name ?? ""} src={user?.avatar_url} className="size-20 text-xl" />
        <span>{user?.name}</span>
      </div>
      <form onSubmit={onSubmit} noValidate aria-busy={pending} className="flex flex-col gap-4">
        <div className="flex flex-col gap-2">
          <Label htmlFor="avatar">Choose picture</Label>
          <Input id="avatar" name="avatar" type="file" accept="image/jpeg,image/png,image/webp" disabled={pending} aria-describedby="avatar-help avatar-error" aria-invalid={!!error} onChange={() => setError("")} />
          <p id="avatar-help" className="text-sm text-muted-foreground">JPEG, PNG or WebP. Resized to fit within 512 × 512 pixels before upload. Transparency is saved on white.</p>
          <p id="avatar-error" role="alert" className="text-sm text-destructive">{error}{wait > 0 && ` Try again in ${wait}s.`}</p>
        </div>
        <Button type="submit" disabled={pending || wait > 0}>{pending ? "Uploading…" : "Save picture"}</Button>
      </form>
    </main>
  );
}
