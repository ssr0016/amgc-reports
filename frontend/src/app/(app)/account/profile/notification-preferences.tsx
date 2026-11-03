"use client";

import { useState } from "react";
import { Bell } from "lucide-react";
import { toast } from "sonner";
import { PanelSection, panelCard } from "@/components/panel-section";
import { Card } from "@/components/ui/card";
import { ApiError } from "@/lib/api";
import { updateDailyDigest } from "@/lib/auth";
import { useSession } from "@/lib/session";

export function NotificationPreferences() {
  const { user, updateUser } = useSession();
  const [pending, setPending] = useState(false);
  const [enabled, setEnabled] = useState<boolean | null>(null);
  if (!user) return null;

  const checked = enabled ?? user.daily_digest;

  async function onChange(enabled: boolean) {
    if (pending) return;
	const previous = checked;
	setEnabled(enabled);
	setPending(true);
    try {
      const saved = await updateDailyDigest(enabled);
      updateUser(saved);
	  setEnabled(saved.daily_digest);
      toast.success("Daily digest preference updated");
    } catch (error) {
	  setEnabled(previous);
      toast.error(error instanceof ApiError ? error.message : "Could not update daily digest preference.");
    } finally {
      setPending(false);
    }
  }

  return <Card className={panelCard}>
    <PanelSection title="Notifications" icon={Bell} description="Receive the current month's report status every morning at 8:00 AM.">
      <label className="flex min-h-11 items-center justify-between gap-4 text-sm font-medium" htmlFor="daily-digest">
        <span>Daily digest</span>
		<input id="daily-digest" type="checkbox" role="switch" className="size-5 accent-[var(--primary)]" checked={checked} disabled={pending} onChange={(event) => void onChange(event.target.checked)} />
      </label>
    </PanelSection>
  </Card>;
}
