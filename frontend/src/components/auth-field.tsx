"use client";

import { Eye, EyeOff, type LucideIcon } from "lucide-react";
import { cn } from "cn";
import { useState } from "react";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

export function AuthField({
  name,
  label,
  error,
  hint,
  type,
  defaultShown = false,
  icon: Icon,
  ...props
}: React.ComponentProps<"input"> & { name: string; label: string; error?: string; hint?: string; defaultShown?: boolean; icon?: LucideIcon }) {
  const [shown, setShown] = useState(defaultShown);
  const isPassword = type === "password";
  const note = error ?? hint;
  const input = (
    <Input
      id={name}
      name={name}
      type={isPassword && shown ? "text" : type}
      aria-invalid={!!error}
      aria-describedby={note ? `${name}-note` : undefined}
      className={cn(Icon && "pl-10", isPassword && "pr-12")}
      {...props}
    />
  );
  return (
    <div className="flex flex-col gap-2">
      <Label htmlFor={name}>{label}</Label>
      <div className="relative">
        {Icon && (
          <Icon aria-hidden="true" className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
        )}
        {input}
        {isPassword && (
          <button
            type="button"
            onClick={() => setShown(!shown)}
            aria-label="Show password"
            aria-pressed={shown}
            className="absolute inset-y-0 right-0 flex min-h-11 w-11 cursor-pointer items-center justify-center rounded-r-sm text-muted-foreground outline-none hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50"
          >
            {shown ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
          </button>
        )}
      </div>
      {note && (
        <p id={`${name}-note`} className={`text-sm ${error ? "text-destructive" : "text-muted-foreground"}`}>
          {note}
        </p>
      )}
    </div>
  );
}
