import type { ReactNode } from "react";
import { cn } from "cn";

// Every app page's <main>: the skip-link target, centred, with the same gutters. Pass a max-w-* or theme class to vary it.
export function PageShell({ className, children }: { className?: string; children: ReactNode }) {
  return (
    <main id="main-content" tabIndex={-1} className={cn("mx-auto flex w-full max-w-5xl flex-1 flex-col gap-6 p-4 sm:p-6 focus:outline-none", className)}>
      {children}
    </main>
  );
}

// The page title in sentence case, an optional description, and any actions on the right.
export function PageHeader({
  title,
  description,
  heading: Heading = "h1",
  children,
}: {
  title: ReactNode;
  description?: ReactNode;
  heading?: "h1" | "h2";
  children?: ReactNode;
}) {
  return (
    <header className="flex flex-wrap items-start justify-between gap-4">
      <div>
        <Heading className="text-2xl font-semibold tracking-tight">{title}</Heading>
        {description && <p className="mt-2 text-sm text-muted-foreground">{description}</p>}
      </div>
      {children && <div className="flex flex-wrap items-center gap-2">{children}</div>}
    </header>
  );
}
