import { cn } from "cn";
import { statusLabel } from "@/lib/user-status";

// The label is always printed, so colour is never the only cue (WCAG 1.4.1). Each pair is a
// light tint with dark text, and the reverse in the dark theme, to stay above 4.5:1 for axe.
// A plain span with its own colours: Badge's default background would compete with these.
const TONE: Record<string, string> = {
  active: "bg-primary/10 text-primary",
  inactive: "bg-gray-200 text-gray-900 dark:bg-gray-800 dark:text-gray-100",
  disciplinary:
    "bg-yellow-100 text-yellow-900 dark:bg-yellow-950 dark:text-yellow-200",
  deceased:
    "bg-neutral-900 text-neutral-50 dark:bg-neutral-100 dark:text-neutral-900",
};

export function UserStatusBadge({ status }: { status: string }) {
  return (
    <span
      data-testid="user-status"
      data-status={status}
      className={cn(
        "inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium whitespace-nowrap ring-1 ring-current/15 ring-inset",
        TONE[status] ?? "bg-muted text-foreground",
      )}
    >
      <span aria-hidden="true" className="size-1.5 rounded-full bg-current" />
      {statusLabel(status)}
    </span>
  );
}
