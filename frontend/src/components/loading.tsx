import { Loader2 } from "lucide-react";

// The one loading line for tabs and dialogs. The spinner is decoration; the text is what gets announced.
export function Loading({ children = "Loading…" }: { children?: React.ReactNode }) {
  return (
    <p role="status" className="flex items-center gap-2 text-sm text-muted-foreground">
      <Loader2 aria-hidden="true" className="size-4 motion-safe:animate-spin" />
      {children}
    </p>
  );
}
