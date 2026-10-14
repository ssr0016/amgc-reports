import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

export function AuthField({
  name,
  label,
  error,
  hint,
  ...props
}: React.ComponentProps<"input"> & { name: string; label: string; error?: string; hint?: string }) {
  const note = error ?? hint;
  return (
    <div className="flex flex-col gap-2">
      <Label htmlFor={name}>{label}</Label>
      <Input
        id={name}
        name={name}
        aria-invalid={!!error}
        aria-describedby={note ? `${name}-note` : undefined}
        {...props}
      />
      {note && (
        <p id={`${name}-note`} className={`text-sm ${error ? "text-destructive" : "text-muted-foreground"}`}>
          {note}
        </p>
      )}
    </div>
  );
}
