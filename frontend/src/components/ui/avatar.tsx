"use client";

import { useState, useSyncExternalStore } from "react";
import { cn } from "cn";

function initials(name: string) {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  const first = parts[0]?.[0] ?? "";
  const last = parts.length > 1 ? parts[parts.length - 1][0] : "";
  return (first + last).toUpperCase() || "?";
}

// I-force ang client-side render gamit ang useSyncExternalStore
const emptySubscribe = () => () => {};
const useMounted = () =>
  useSyncExternalStore(
    emptySubscribe,
    () => true,
    () => false,
  );

export function Avatar({
  name,
  src,
  className,
}: {
  name: string;
  src?: string;
  className?: string;
}) {
  const mounted = useMounted();
  const [failedSrc, setFailedSrc] = useState<string | null>(null);

  const showImage = mounted && src && src !== failedSrc;

  return (
    <span
      aria-hidden="true"
      className={cn(
        "relative inline-flex size-8 shrink-0 items-center justify-center overflow-hidden rounded-full bg-secondary text-xs font-medium text-secondary-foreground select-none",
        className,
      )}
    >
      <span>{initials(name)}</span>
      {showImage && (
        // Public avatars load directly from R2 without the Next.js image loader.
        // eslint-disable-next-line @next/next/no-img-element
        <img
          src={src}
          alt=""
          width={512}
          height={512}
          className="absolute inset-0 size-full object-cover"
          onError={() => setFailedSrc(src)}
        />
      )}
    </span>
  );
}
