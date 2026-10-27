"use client";

import { useState } from "react";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";
import { dashboardTheme } from "@/components/dashboard-theme";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Avatar } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ApiError } from "@/lib/api";
import { resizeAvatar, uploadAvatar } from "@/lib/avatar";
import { useCountdown } from "@/lib/auth-form";
import { queryClient } from "@/lib/query";
import { useSession } from "@/lib/session";
import { ProfileSkeleton } from "@/components/loading-skeletons";

export function ProfilePicture() {
  const { user, updateAvatar } = useSession();
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const [wait, setWait] = useCountdown();

  if (!user) return <main id="main-content" className={`${dashboardTheme} account-surface mx-auto flex w-full max-w-3xl flex-1 flex-col gap-6 p-4 sm:p-6`}><h1 className="text-2xl font-medium tracking-tight">Profile</h1><ProfileSkeleton /></main>;

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
    <main id="main-content" tabIndex={-1} className={`${dashboardTheme} account-surface mx-auto flex w-full max-w-3xl flex-1 flex-col gap-6 p-4 sm:p-6 focus:outline-none`}>
      <h1 className="text-2xl font-medium tracking-tight">Profile</h1>
      <Card className="p-4 sm:p-6">
        <h2 className="text-lg font-medium tracking-tight">Account details</h2>
        <p className="text-sm text-muted-foreground">These details are managed by your admin.</p>
        <div className="grid min-w-0 gap-4 sm:grid-cols-2">
          {[["Name", user?.name], ["Email", user?.email], ["Role", user?.role], ["Area assignment", user?.area_assignment], ["Church", user?.church_name]].map(([label, value]) => (
            <div key={label} className="flex min-w-0 flex-col gap-2">
              <Label htmlFor={`profile-${label?.replaceAll(" ", "-")}`}>{label}</Label>
              <Input id={`profile-${label?.replaceAll(" ", "-")}`} value={value ?? ""} readOnly />
            </div>
          ))}
        </div>
      </Card>
      <Card className="gap-0 rounded-lg py-0">
        <CardHeader className="gap-4 border-b p-6">
          <div className="flex flex-wrap items-center gap-4">
            <Avatar name={user?.name ?? ""} src={user?.avatar_url} className="size-20 bg-secondary! text-secondary-foreground! cursor-default text-xl" />
            <div className="min-w-0">
              <CardTitle><h2 className="break-words">{user?.name || "Your account"}</h2></CardTitle>
              <p className="mt-2 text-sm text-muted-foreground">Update the picture shown with your account.</p>
            </div>
          </div>
        </CardHeader>
        <CardContent className="p-6">
          <form onSubmit={onSubmit} noValidate aria-busy={pending} className="flex flex-col gap-5">
            <div className="flex flex-col gap-2">
              <Label htmlFor="avatar">Choose picture</Label>
              <Input id="avatar" name="avatar" type="file" accept="image/jpeg,image/png,image/webp" disabled={pending} aria-describedby="avatar-help avatar-error" aria-invalid={!!error} onChange={() => setError("")} />
              <p id="avatar-help" className="text-sm text-muted-foreground">JPEG, PNG or WebP. Resized to fit within 512 × 512 pixels before upload. Transparency is saved on white.</p>
              <p id="avatar-error" role="alert" className="text-sm text-destructive">{error}{wait > 0 && ` Try again in ${wait}s.`}</p>
            </div>
            <Button type="submit" className="min-h-11 self-start px-4" disabled={pending || wait > 0}>{pending ? <><Loader2 aria-hidden="true" className="motion-safe:animate-spin" />Uploading…</> : "Save picture"}</Button>
          </form>
        </CardContent>
      </Card>
    </main>
  );
}
