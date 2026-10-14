"use client";

import { Toaster as Sonner, type ToasterProps } from "sonner";

// No next-themes here: the app has no theme provider.
export function Toaster(props: ToasterProps) {
  return <Sonner richColors closeButton {...props} />;
}
