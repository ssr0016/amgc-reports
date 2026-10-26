"use client";

import { Eye, EyeOff } from "lucide-react";
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
  ...props
}: React.ComponentProps<"input"> & { name: string; label: string; error?: string; hint?: string; defaultShown?: boolean }) {
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
      className={isPassword ? "pr-12" : undefined}
      {...props}
    />
  );
  return (
    <div className="flex flex-col gap-2">
      <Label htmlFor={name}>{label}</Label>
      {isPassword ? (
        <div className="relative">
          {input}
          <button
            type="button"
            onClick={() => setShown(!shown)}
            aria-label="Show password"
            aria-pressed={shown}
            className="absolute inset-y-0 right-0 flex min-h-11 w-11 cursor-pointer items-center justify-center rounded-r-sm text-muted-foreground outline-none hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50"
          >
            {shown ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
          </button>
        </div>
      ) : (
        input
      )}
      {note && (
        <p id={`${name}-note`} className={`text-sm ${error ? "text-destructive" : "text-muted-foreground"}`}>
          {note}
        </p>
      )}
    </div>
  );
}
