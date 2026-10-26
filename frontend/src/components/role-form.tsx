"use client";

import { useState } from "react";
import { AuthField } from "@/components/auth-field";
import { Button } from "@/components/ui/button";
import { ApiError } from "@/lib/api";
import { toFormError } from "@/lib/auth-form";
import { useFocusFirstError } from "@/lib/form-focus";
import { actionWords, groupPermissions, type PermissionInfo, type RoleInput } from "@/lib/roles";

// Create and edit share this form. Saving sends the whole set: the name, the
// description and every checked permission (PUT replaces them all). A built-in role
// opens read-only, with a note saying why.
export function RoleForm({
  initial = { name: "", description: "", permission_ids: [] },
  permissions,
  readOnly = false,
  submitLabel,
  onSubmit,
  onCancel,
}: {
  initial?: RoleInput;
  permissions: PermissionInfo[];
  readOnly?: boolean;
  submitLabel: string;
  onSubmit: (values: RoleInput) => Promise<void>;
  onCancel: () => void;
}) {
  const [form, setForm] = useState(initial);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [general, setGeneral] = useState("");
  const [pending, setPending] = useState(false);
  const [formRef, focusError] = useFocusFirstError<HTMLFormElement>();
  const checked = new Set(form.permission_ids);

  const clearError = (name: string) =>
    setErrors((prev) => Object.fromEntries(Object.entries(prev).filter(([k]) => k !== name)));

  function setField(name: "name" | "description", value: string) {
    setForm((f) => ({ ...f, [name]: value }));
    clearError(name);
  }

  function toggle(id: number) {
    setForm((f) => ({
      ...f,
      permission_ids: checked.has(id) ? f.permission_ids.filter((p) => p !== id) : [...f.permission_ids, id],
    }));
    clearError("permission_ids");
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setErrors({});
    setGeneral("");
    setPending(true);
    try {
      await onSubmit({ ...form, name: form.name.trim(), description: (form.description ?? "").trim() });
    } catch (err) {
      setPending(false);
      // A duplicate name, a built-in role or a role in use: the server's message says which.
      if (err instanceof ApiError && err.isConflict) {
        setGeneral(err.message);
        return;
      }
      const failure = toFormError(err);
      setErrors(failure.fields);
      setGeneral(failure.message || "Fix the highlighted fields and try again.");
      focusError();
    }
  }

  return (
    <form ref={formRef} onSubmit={submit} className="flex w-full flex-col gap-6 rounded-lg border bg-card p-4 sm:p-6">
      {readOnly && (
        <p className="text-sm text-muted-foreground">
          This is a built-in role. The app depends on its name and permissions, so it cannot be changed.
        </p>
      )}
      <AuthField
        name="name"
        label="Name"
        required
        minLength={2}
        maxLength={50}
        disabled={readOnly}
        value={form.name}
        onChange={(e) => setField("name", e.target.value)}
        error={errors.name}
      />
      <AuthField
        name="description"
        label="Description"
        maxLength={255}
        disabled={readOnly}
        value={form.description ?? ""}
        onChange={(e) => setField("description", e.target.value)}
        error={errors.description}
      />
      <div className="grid items-start gap-4 lg:grid-cols-2">
        {groupPermissions(permissions).map((group) => (
          <fieldset key={group.label} className="flex min-w-0 flex-col gap-2 rounded-lg border p-4" disabled={readOnly}>
            <legend className="px-2 font-medium">{group.label}</legend>
            {group.permissions.map((p) => (
              <label key={p.id} htmlFor={`perm-${p.id}`} className="permission-option text-sm">
                <input
                  id={`perm-${p.id}`}
                  type="checkbox"
                  className="size-5 shrink-0 accent-[var(--approved)]"
                  checked={checked.has(p.id)}
                  onChange={() => toggle(p.id)}
                />
                <span className="flex min-w-0 flex-col gap-1">
                  <span className="font-medium">{actionWords(p.action)}</span>
                  <code className="break-all font-mono text-xs text-muted-foreground">{p.name}</code>
                </span>
              </label>
            ))}
          </fieldset>
        ))}
      </div>
      {errors.permission_ids && (
        <p id="permission_ids-note" data-field-error tabIndex={-1} className="text-sm text-destructive">
          Permissions: {errors.permission_ids}
        </p>
      )}
      {general && (
        <p role="alert" className="text-sm text-destructive">
          {general}
        </p>
      )}
      <div className="flex flex-wrap gap-3 border-t pt-4 [&_button]:min-h-11">
        {!readOnly && (
          <Button type="submit" disabled={pending}>
            {pending ? "Saving…" : submitLabel}
          </Button>
        )}
        <Button type="button" variant="outline" onClick={onCancel} disabled={pending}>
          {readOnly ? "Back" : "Cancel"}
        </Button>
      </div>
    </form>
  );
}
