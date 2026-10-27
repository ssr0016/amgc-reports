import * as React from "react";
import { cn } from "cn";

function Skeleton({ className, ...props }: React.ComponentProps<"span">) {
  return <span aria-hidden="true" data-slot="skeleton" className={cn("animate-pulse rounded-md bg-muted", className)} {...props} />;
}

export { Skeleton };
