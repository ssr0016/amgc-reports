"use client";

import { useState } from "react";
import { FileText, KeyRound, ListChecks, Loader2, ScrollText, ShieldCheck, Users, type LucideIcon } from "lucide-react";
import { AuthField } from "@/components/auth-field";
import { PanelSection, panelCard } from "@/components/panel-section";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { ApiError } from "@/lib/api";
import { toFormError } from "@/lib/auth-form";
import { useFocusFirstError } from "@/lib/form-focus";
import { actionWords, groupPermissions, type PermissionInfo, type RoleInput } from "@/lib/roles";

// Keyed by groupPermissions' labels; any other resource falls back to ShieldCheck.
const GROUP_STYLE: Record<string, { icon: LucideIcon; tile: string }> = {
  Reports: { icon: FileText, tile: "bg-primary/10 text-primary" },
  Users: { icon: Users, tile: "bg-sky-500/10 text-sky-400" },
  Admin: { icon: KeyRound, tile: "bg-amber-500/10 text-amber-400" },
  // The sidebar's icons for Logs and Tracker.
  Audit: { icon: ScrollText, tile: "bg-violet-500/10 text-violet-400" },
  Tracker: { icon: ListChecks, tile: "bg-teal-500/10 text-teal-400" },
};

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
    <form ref={formRef} onSubmit={submit} className="w-full">
      <Card className={panelCard}>
        <PanelSection
          title="Role details"
          icon={ShieldCheck}
          description={
            readOnly
              ? "This is a built-in role. The app depends on its name and permissions, so it cannot be changed."
              : "How this role appears in role lists and dropdowns."
          }
          className="border-b"
        >
          <div className="grid gap-5 sm:grid-cols-2">
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
          </div>
        </PanelSection>
        {groupPermissions(permissions).map((group) => {
          const style = GROUP_STYLE[group.label] ?? { icon: ShieldCheck, tile: "bg-muted text-muted-foreground" };
          const on = group.permissions.filter((p) => checked.has(p.id)).length;
          return (
            <PanelSection
              key={group.label}
              title={group.label}
              icon={style.icon}
              tile={style.tile}
              description={`${on} of ${group.permissions.length} selected`}
              className="border-b"
            >
              <div className="grid gap-3 sm:grid-cols-2">
                {group.permissions.map((p) => (
                  <label key={p.id} htmlFor={`perm-${p.id}`} className="permission-option text-sm">
                    <input
                      id={`perm-${p.id}`}
                      type="checkbox"
                      className="mt-0.5 size-5 shrink-0 accent-[var(--approved)]"
                      disabled={readOnly}
                      checked={checked.has(p.id)}
                      onChange={() => toggle(p.id)}
                    />
                    <span className="flex min-w-0 flex-col gap-1">
                      <span className="font-medium">{actionWords(p.action)}</span>
                      <code className="break-all font-mono text-xs text-muted-foreground">{p.name}</code>
                    </span>
                  </label>
                ))}
              </div>
            </PanelSection>
          );
        })}
        {(errors.permission_ids || general) && (
          <div className="flex flex-col gap-2 p-5 sm:px-6">
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
          </div>
        )}
        {/* Same footer as UserForm: the primary action on top on phones, on the right on wider screens. */}
        <div className="flex flex-col-reverse gap-2 bg-muted/30 p-5 sm:flex-row sm:justify-end sm:p-6">
          <Button type="button" variant="outline" className="min-h-11 px-4" onClick={onCancel} disabled={pending}>
            {readOnly ? "Back" : "Cancel"}
          </Button>
          {!readOnly && (
            <Button type="submit" className="min-h-11 px-4" disabled={pending}>
              {pending ? <><Loader2 aria-hidden="true" className="motion-safe:animate-spin" />Saving…</> : submitLabel}
            </Button>
          )}
        </div>
      </Card>
    </form>
  );
}
