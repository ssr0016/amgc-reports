"use client";

import { useEffect, useState } from "react";
import { Loader2 } from "lucide-react";
import { useFocusFirstError } from "@/lib/form-focus";
import { AuthField } from "@/components/auth-field";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { NativeSelect } from "@/components/ui/native-select";
import { ApiError } from "@/lib/api";
import { toFormError, useCountdown } from "@/lib/auth-form";
import type { Role } from "@/lib/users";

export type UserFormValues = {
  email: string;
  name: string;
  password: string;
  confirm_password: string;
  role_id: string;
  area_assignment: string;
  church_name: string;
};

export const emptyUser: UserFormValues = {
  email: "",
  name: "",
  password: "",
  confirm_password: "",
  role_id: "",
  area_assignment: "",
  church_name: "",
};

// Create asks for an email, password and role; edit shows the email read-only and
// changes only name, area and church. A failed save leaves what was typed in place.
export function UserForm({
  mode,
  initial = emptyUser,
  roles = [],
  submitLabel,
  onSubmit,
  onCancel,
}: {
  mode: "create" | "edit";
  initial?: UserFormValues;
  // Empty when the roles could not be loaded: the select is hidden and the server picks the default.
  roles?: Role[];
  submitLabel: string;
  onSubmit: (values: UserFormValues) => Promise<void>;
  onCancel: () => void;
}) {
  const create = mode === "create";
  const [seed] = useState(() => ({
    ...initial,
    role_id: initial.role_id || String(roles.find((r) => r.name === "user")?.id ?? ""),
  }));
  const [form, setForm] = useState(seed);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [general, setGeneral] = useState("");
  const [pending, setPending] = useState(false);
  const [saved, setSaved] = useState(false);
  const [wait, setWait] = useCountdown();
  const [formRef, focusError] = useFocusFirstError<HTMLFormElement>();
  const dirty = !saved && JSON.stringify(form) !== JSON.stringify(seed);

  // Covers closing the tab or refreshing; in-app navigation is guarded by Cancel.
  useEffect(() => {
    if (!dirty) return;
    const warn = (e: BeforeUnloadEvent) => e.preventDefault();
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, [dirty]);

  function setField(name: keyof UserFormValues, value: string) {
    setForm((f) => ({ ...f, [name]: value }));
    setErrors((prev) => Object.fromEntries(Object.entries(prev).filter(([k]) => k !== name)));
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setErrors({});
    setGeneral("");
    // Same-twice is checked here so a typo spends no request; the server checks it too.
    // An empty confirmation never gets here: the input is required, so the browser stops it first.
    if (create && form.confirm_password !== form.password) {
      setErrors({ confirm_password: "Confirm password does not match" });
      setGeneral("Fix the highlighted fields and try again.");
      focusError();
      return;
    }
    setPending(true);
    try {
      await onSubmit(form);
      setSaved(true);
    } catch (err) {
      setPending(false);
      if (err instanceof ApiError && err.isConflict) {
        setErrors({ email: "That email is already in use." });
        focusError();
        return;
      }
      const failure = toFormError(err);
      setWait(failure.retryAfter ?? 0);
      const { confirm_password: confirmError, ...rest } = failure.fields;
      setErrors(confirmError ? { ...rest, confirm_password: `Confirm password ${confirmError}` } : rest);
      setGeneral(failure.message || "Fix the highlighted fields and try again.");
      focusError();
    }
  }

  function cancel() {
    if (!dirty || confirm("Discard your changes?")) onCancel();
  }

  return (
    <form ref={formRef} onSubmit={submit} className="flex max-w-xl flex-col gap-4">
      {!create && <p className="text-sm text-muted-foreground">Only name, area, and church can be edited.</p>}
      <AuthField
        name="email"
        label="Email"
        type="email"
        autoComplete="off"
        required={create}
        disabled={!create}
        value={form.email}
        onChange={(e) => setField("email", e.target.value)}
        error={errors.email}
      />
      <AuthField
        name="name"
        label="Name"
        required
        value={form.name}
        onChange={(e) => setField("name", e.target.value)}
        error={errors.name}
      />
      {create && (
        <AuthField
          name="password"
          label="Password"
          type="password"
          autoComplete="new-password"
          required
          value={form.password}
          onChange={(e) => setField("password", e.target.value)}
          error={errors.password}
          hint="Use a long, random password (at least 60 bits of entropy)."
        />
      )}
      {create && (
        <AuthField
          name="confirm_password"
          label="Confirm password"
          type="password"
          autoComplete="new-password"
          required
          value={form.confirm_password}
          onChange={(e) => setField("confirm_password", e.target.value)}
          error={errors.confirm_password}
        />
      )}
      {create && roles.length > 0 && (
        <div className="flex flex-col gap-2">
          <Label htmlFor="role_id">Role</Label>
          <NativeSelect
            id="role_id"
            value={form.role_id}
            onChange={(e) => setField("role_id", e.target.value)}
            aria-invalid={!!errors.role_id}
            aria-describedby={errors.role_id ? "role_id-note" : undefined}
          >
            {roles.map((r) => (
              <option key={r.id} value={r.id}>
                {r.name}
              </option>
            ))}
          </NativeSelect>
          {errors.role_id && (
            <p id="role_id-note" className="text-sm text-destructive">
              {errors.role_id}
            </p>
          )}
        </div>
      )}
      <AuthField
        name="area_assignment"
        label="Area Assignment"
        value={form.area_assignment}
        onChange={(e) => setField("area_assignment", e.target.value)}
        error={errors.area_assignment}
      />
      <AuthField
        name="church_name"
        label="Church Name"
        value={form.church_name}
        onChange={(e) => setField("church_name", e.target.value)}
        error={errors.church_name}
      />
      {general && (
        <p role="alert" className="text-sm text-destructive">
          {general}
          {wait > 0 && ` Try again in ${wait}s.`}
        </p>
      )}
      <div className="flex flex-col gap-2 sm:flex-row">
        <Button type="submit" disabled={pending || wait > 0}>
          {pending ? <><Loader2 aria-hidden="true" className="motion-safe:animate-spin" />Saving…</> : submitLabel}
        </Button>
        <Button type="button" variant="outline" onClick={cancel} disabled={pending}>
          Cancel
        </Button>
      </div>
    </form>
  );
}
