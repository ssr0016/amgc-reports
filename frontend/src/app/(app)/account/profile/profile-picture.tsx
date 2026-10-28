"use client";

import { useState } from "react";
import { Camera, Church, IdCard, ImageUp, Loader2, Mail, MapPin, Shield, UserRound } from "lucide-react";
import { toast } from "sonner";
import { AuthField } from "@/components/auth-field";
import { dashboardTheme } from "@/components/dashboard-theme";
import { PanelSection, panelCard } from "@/components/panel-section";
import { Card } from "@/components/ui/card";
import { Avatar } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { ApiError } from "@/lib/api";
import { resizeAvatar, uploadAvatar } from "@/lib/avatar";
import { useCountdown } from "@/lib/auth-form";
import { queryClient } from "@/lib/query";
import { useSession } from "@/lib/session";
import { ProfileSkeleton } from "@/components/loading-skeletons";
import { PageHeader, PageShell } from "@/components/page-shell";

export function ProfilePicture() {
  const { user, updateAvatar } = useSession();
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const [wait, setWait] = useCountdown();
  const [fileName, setFileName] = useState("");

  const header = <PageHeader title="Profile" description="Your account details and profile picture." />;

  if (!user) return <PageShell className={`${dashboardTheme} max-w-3xl`}>{header}<ProfileSkeleton /></PageShell>;

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
      setFileName("");
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

  const details = [
    ["name", "Name", user.name, UserRound],
    ["email", "Email", user.email, Mail],
    ["role", "Role", user.role, Shield],
    ["area", "Area assignment", user.area_assignment, MapPin],
    ["church", "Church", user.church_name, Church],
  ] as const;

  return (
    <PageShell className={`${dashboardTheme} max-w-3xl`}>
      {header}
      <Card className={panelCard}>
        <PanelSection title="Account details" icon={IdCard} description="These details are managed by your admin.">
          <div className="grid min-w-0 gap-4 sm:grid-cols-2">
            {details.map(([key, label, value, icon]) => (
              <AuthField key={key} name={`profile-${key}`} label={label} icon={icon} value={value ?? ""} readOnly />
            ))}
          </div>
        </PanelSection>
      </Card>
      <form onSubmit={onSubmit} noValidate aria-busy={pending}>
        <Card className={panelCard}>
          <PanelSection title="Profile picture" icon={Camera} tile="bg-sky-500/10 text-sky-400" description="Update the picture shown with your account.">
            <div className="flex flex-col gap-5 sm:flex-row sm:items-center">
              <Avatar name={user.name ?? ""} src={user.avatar_url} className="size-24 shrink-0 cursor-default bg-secondary! text-2xl text-secondary-foreground! ring-2 ring-primary/30 ring-offset-4 ring-offset-card" />
              <div className="flex min-w-0 flex-1 flex-col gap-2">
                <Label htmlFor="avatar">Choose picture</Label>
                {/* The real file input covers the drop zone, so click, keyboard and native drag-and-drop all work. */}
                <div className="relative flex min-h-24 flex-col items-center justify-center gap-1 rounded-lg border border-dashed border-border bg-muted/20 p-4 text-center transition-colors hover:border-primary/50 hover:bg-primary/5 focus-within:border-primary focus-within:ring-3 focus-within:ring-ring/50 has-disabled:opacity-50 has-[[aria-invalid=true]]:border-destructive">
                  <ImageUp aria-hidden="true" className="size-6 text-muted-foreground" />
                  <p className="text-sm font-medium break-all">{fileName || "Click to choose, or drag an image here"}</p>
                  <input id="avatar" name="avatar" type="file" accept="image/jpeg,image/png,image/webp" disabled={pending} aria-describedby="avatar-help avatar-error" aria-invalid={!!error} onChange={(e) => { setError(""); setFileName(e.currentTarget.files?.[0]?.name ?? ""); }} className="absolute inset-0 cursor-pointer opacity-0 disabled:cursor-not-allowed" />
                </div>
                <p id="avatar-help" className="text-sm text-muted-foreground">JPEG, PNG or WebP. Resized to fit within 512 × 512 pixels before upload. Transparency is saved on white.</p>
                <p id="avatar-error" role="alert" className="text-sm text-destructive">{error}{wait > 0 && ` Try again in ${wait}s.`}</p>
              </div>
            </div>
          </PanelSection>
          <div className="flex justify-end border-t bg-muted/30 p-5 sm:p-6">
            <Button type="submit" className="min-h-11 px-4" disabled={pending || wait > 0}>{pending ? <><Loader2 aria-hidden="true" className="motion-safe:animate-spin" />Uploading…</> : "Save picture"}</Button>
          </div>
        </Card>
      </form>
    </PageShell>
  );
}
