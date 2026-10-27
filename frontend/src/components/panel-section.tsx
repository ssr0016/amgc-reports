import { useId, type ReactNode } from "react";
import type { LucideIcon } from "lucide-react";
import { cn } from "cn";

// The StatCard / Users table surface, for form cards and action panels.
export const panelCard = "gap-0 border-white/[0.06] bg-card/80 bg-linear-to-b from-white/[0.03] to-transparent py-0 shadow-sm";

// A titled group inside a panel card. A fieldset, so the legend names the group for screen readers;
// the legend floats so it lays out like any other row instead of sitting on the border.
export function PanelSection({
  title,
  description,
  icon: Icon,
  tile = "bg-primary/10 text-primary",
  className,
  children,
}: {
  title: string;
  description?: ReactNode;
  icon: LucideIcon;
  tile?: string;
  className?: string;
  children: ReactNode;
}) {
  const descId = useId();
  return (
    <fieldset aria-describedby={description ? descId : undefined} className={cn("flex min-w-0 flex-col gap-5 p-5 sm:p-6", className)}>
      <legend className="float-left flex w-full items-center gap-3 text-base font-medium">
        <span aria-hidden="true" className={cn("grid size-9 shrink-0 place-items-center rounded-lg", tile)}>
          <Icon className="size-5" />
        </span>
        {title}
      </legend>
      {description && (
        <p id={descId} className="-mt-3 text-sm text-muted-foreground">
          {description}
        </p>
      )}
      {children}
    </fieldset>
  );
}
