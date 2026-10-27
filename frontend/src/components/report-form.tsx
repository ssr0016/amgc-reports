"use client";

import { memo, useCallback, useEffect, useRef, useState } from "react";
import type { ReactNode } from "react";
import { Loader2 } from "lucide-react";
import { useFocusFirstError } from "@/lib/form-focus";
import { AuthField } from "@/components/auth-field";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { NativeSelect } from "@/components/ui/native-select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Textarea } from "@/components/ui/textarea";
import { ApiError } from "@/lib/api";
import { toFormError, useCountdown } from "@/lib/auth-form";
import {
  fromServerDetails,
  METRIC_SECTIONS,
  metricError,
  metricErrorKey,
  METRICS,
  TEXT_LIMITS,
  RESTRICTED_METRIC,
  toInput,
  validate,
  WEEKS,
  type FormState,
} from "@/lib/report-form";
import { MONTHS, type ReportInput } from "@/lib/reports";

// [name, label, placeholder, help line under the field]
const longTexts = [
  ["names", "Names of New Believers", "Juan dela Cruz, Maria Santos", "e.g., Juan dela Cruz, Maria Santos (Separate names with commas)."],
  [
    "narrative_report",
    "Narrative Report",
    "This month we…",
    "Describe what happened this month in your ministry. Include key events, milestones, or testimonies.",
  ],
  ["challenges", "Challenges", "e.g. Low attendance during the rainy season", "What challenges or difficulties did you encounter this month?"],
  ["prayer_request", "Prayer Request", "e.g. Pray for the youth camp", "List your prayer requests here so the team can pray for you."],
] as const;

const sectionClass = "flex min-w-0 flex-col gap-5 rounded-lg border bg-card p-4 sm:p-6";
const headingClass = "text-lg font-medium tracking-tight";

export function ReportForm({
  initial,
  submitLabel,
  draft = false,
  onAutoSave,
  storageKey,
  revision = false,
  lockedPeriod,
  readOnly = false,
  restricted = false,
  onSubmit,
  onCancel,
  endAction,
}: {
  initial: FormState;
  submitLabel: string;
  draft?: boolean;
  onAutoSave?: (input: ReportInput) => Promise<{ id?: number } | void>;
  storageKey?: string;
  revision?: boolean;
  // When set, month and year cannot be changed and must match it.
  lockedPeriod?: { month: string; year: number };
  // Shows the report without letting anything change: every field is disabled and there is no Save.
  readOnly?: boolean;
  // The server withheld the free text and tithes_offering (D-18): show "Restricted" instead of empty fields.
  restricted?: boolean;
  onSubmit: (input: ReportInput) => Promise<void>;
  onCancel: () => void;
  endAction?: ReactNode;
}) {
  const [start] = useState(initial);
  const [form, setForm] = useState(initial);
  const [userNotes, setUserNotes] = useState("");
  // Keys of the fields that differ from start, so dirty is not a whole-form compare per keystroke.
  const [changed, setChanged] = useState<ReadonlySet<string>>(() => new Set());
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [general, setGeneral] = useState("");
  const [pending, setPending] = useState(false);
  const [saved, setSaved] = useState(false);
  const [hydrated, setHydrated] = useState(!storageKey || readOnly);
  const [recovered, setRecovered] = useState(false);
  const [lastAutoSaved, setLastAutoSaved] = useState<Date | null>(null);
  const skipStorageWrite = useRef(false);
  const [wait, setWait] = useCountdown();
  const [formRef, focusError] = useFocusFirstError<HTMLFormElement>();
  const dirty = !saved && changed.size > 0;

  useEffect(() => {
    if (!storageKey || readOnly) return;
    try {
      const raw = localStorage.getItem(storageKey);
      if (raw) {
        const data = JSON.parse(raw) as { context?: string; form?: FormState; userNotes?: string };
        if (data.context !== storageKey) {
          localStorage.removeItem(storageKey);
        } else if (data.form) {
          // eslint-disable-next-line react-hooks/set-state-in-effect -- localStorage is only available after client hydration.
          setForm(data.form);
          setUserNotes(data.userNotes ?? "");
          setChanged(new Set(["recovered"]));
          setRecovered(true);
        }
      }
    } catch {
      localStorage.removeItem(storageKey);
    }
    setHydrated(true);
  }, [readOnly, storageKey]);

  useEffect(() => {
    if (!storageKey || readOnly || !hydrated || !changed.size) return;
    if (skipStorageWrite.current) {
      skipStorageWrite.current = false;
      return;
    }
    localStorage.setItem(storageKey, JSON.stringify({ context: storageKey, form, userNotes }));
  }, [changed.size, form, hydrated, readOnly, storageKey, userNotes]);

  useEffect(() => {
    if (!onAutoSave || readOnly || !hydrated || !dirty) return;
    const timer = window.setTimeout(async () => {
      const input = toInput(form);
      if (revision) input.user_notes = userNotes;
      try {
        await onAutoSave(input);
        if (storageKey) localStorage.removeItem(storageKey);
        setLastAutoSaved(new Date());
        setRecovered(false);
        setSaved(true);
      } catch {
        // Local storage remains the recovery path when the background request fails.
      }
    }, 3000);
    return () => window.clearTimeout(timer);
  }, [dirty, form, hydrated, onAutoSave, readOnly, revision, storageKey, userNotes]);

  // Covers closing the tab or refreshing; in-app navigation is guarded by Cancel.
  useEffect(() => {
    if (!dirty) return;
    const warn = (e: BeforeUnloadEvent) => e.preventDefault();
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, [dirty]);

  // Each updater returns prev when nothing changes, so memoised rows skip the re-render.
  const markChanged = useCallback((key: string, isChanged: boolean) => {
    setChanged((prev) => {
      if (prev.has(key) === isChanged) return prev;
      const next = new Set(prev);
      if (isChanged) next.add(key);
      else next.delete(key);
      return next;
    });
  }, []);

  const clearError = useCallback((...keys: string[]) => {
    setErrors((prev) => (keys.some((k) => k in prev) ? Object.fromEntries(Object.entries(prev).filter(([k]) => !keys.includes(k))) : prev));
  }, []);

  function setField(name: keyof Omit<FormState, "metrics">, value: string) {
    setForm((f) => ({ ...f, [name]: value }));
    setSaved(false);
    markChanged(name, value !== start[name]);
    clearError(name);
  }

  const setWeek = useCallback(
    (metric: string, week: (typeof WEEKS)[number], value: string) => {
      setForm((f) => ({ ...f, metrics: { ...f.metrics, [metric]: { ...f.metrics[metric], [week]: value } } }));
      setSaved(false);
      markChanged(metricErrorKey(metric, week), value !== (start.metrics[metric]?.[week] ?? ""));
      clearError(metricErrorKey(metric, week), `metrics.${metric}`);
    },
    [start, markChanged, clearError],
  );

  const checkWeek = useCallback((metric: string, week: (typeof WEEKS)[number], value: string) => {
    const key = metricErrorKey(metric, week);
    const message = metricError(metric, value);
    setErrors((prev) => {
      if (prev[key] === message || (!message && !(key in prev))) return prev;
      const next = { ...prev };
      if (message) next[key] = message;
      else delete next[key];
      return next;
    });
  }, []);

  async function submit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const found = validate(form, lockedPeriod);
    setErrors(found);
    setGeneral(Object.keys(found).length ? "Fix the highlighted fields and try again." : "");
    if (Object.keys(found).length) {
      focusError();
      return;
    }

    setPending(true);
    setSaved(true);
    try {
      const input = toInput(form);
      if (revision) input.user_notes = userNotes;
      await onSubmit(input);
      setPending(false);
      if (storageKey) localStorage.removeItem(storageKey);
      setSaved(true);
    } catch (err) {
      setPending(false);
      setSaved(false);
      if (err instanceof ApiError && err.isConflict) {
        setErrors({ month: "You already have a report for this month and year." });
        setGeneral("");
        focusError();
        return;
      }
      const failure = toFormError(err);
      const mapped = fromServerDetails(failure.fields);
      setWait(failure.retryAfter ?? 0);
      setErrors(mapped.fields);
      setGeneral(failure.message || mapped.general || "Fix the highlighted fields and try again.");
      focusError();
    }
  }

  function cancel() {
    if (!dirty || confirm("Discard your changes?")) onCancel();
  }

  function discardRecovery() {
    skipStorageWrite.current = true;
    if (storageKey) localStorage.removeItem(storageKey);
    setForm(start);
    setUserNotes("");
    setChanged(new Set());
    setRecovered(false);
    setSaved(true);
  }

  const weekErrors = METRICS.flatMap((m) =>
    WEEKS.flatMap((w) => {
      const message = errors[`metrics.${m.key}.${w}`];
      return message ? [`${m.label}, week ${w.slice(4)}: ${message}`] : [];
    }),
  );

  return (
    <form ref={formRef} onSubmit={submit} noValidate className="flex min-w-0 flex-col gap-6">
      <fieldset
        disabled={readOnly}
        className={`flex min-w-0 flex-col gap-8${readOnly ? " [&_:disabled]:opacity-100! [&_select:disabled]:text-foreground!" : ""}`}
      >
        <section aria-labelledby="section-basic" className={sectionClass}>
          <h2 id="section-basic" className={headingClass}>
            Basic Information
          </h2>
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="flex flex-col gap-2">
              <Label htmlFor="month">Month</Label>
              <NativeSelect
                id="month"
                value={form.month}
                onChange={(e) => setField("month", e.target.value)}
                disabled={!!lockedPeriod}
                aria-invalid={!!errors.month}
                aria-describedby={errors.month ? "month-note" : undefined}
              >
                <option value="">Choose a month</option>
                {MONTHS.map((m) => (
                  <option key={m} value={m}>
                    {m}
                  </option>
                ))}
              </NativeSelect>
              {errors.month && (
                <p id="month-note" data-field-error tabIndex={-1} className="text-sm text-destructive focus:outline-none">
                  {errors.month}
                </p>
              )}
            </div>
            <AuthField
              name="year"
              label="Year"
              inputMode="numeric"
              value={form.year}
              onChange={(e) => setField("year", e.target.value)}
              readOnly={!!lockedPeriod}
              hint={lockedPeriod ? "Fixed for this report." : undefined}
              error={errors.year}
            />
            <div className="flex flex-col gap-2 rounded-lg border bg-muted/30 p-4 sm:col-span-2">
              <p className="text-sm font-medium">Worker Profile</p>
              <dl className="grid gap-3 sm:grid-cols-3">
                {([
                  ["Worker Name", form.worker],
                  ["Area Assignment", form.area_assignment],
                  ["Church Name", form.church_name],
                ] as const).map(([label, value]) => (
                  <div key={label}>
                    <dt className="text-xs text-muted-foreground">{label}</dt>
                    <dd>{value || "Not assigned"}</dd>
                  </div>
                ))}
              </dl>
              <p className="text-xs text-muted-foreground">These details come from your profile and cannot be changed in a report.</p>
            </div>
          </div>
        </section>

        {revision && !readOnly && (
          <section className={sectionClass}>
            <h2 className={headingClass}>Revision Notes</h2>
            <Label htmlFor="user-notes">Your Notes / Explanation for Revision</Label>
            <Textarea
              id="user-notes"
              value={userNotes}
              onChange={(e) => {
                setUserNotes(e.target.value);
                setSaved(false);
                markChanged("user_notes", e.target.value !== "");
              }}
              maxLength={5000}
              rows={4}
            />
          </section>
        )}

        {weekErrors.length > 0 && (
          <ul role="alert" className="list-disc pl-5 text-sm text-destructive">
            {weekErrors.map((message) => (
              <li key={message}>{message}</li>
            ))}
          </ul>
        )}

        {METRIC_SECTIONS.map((section, i) => {
          const id = `section-${section.title.toLowerCase().replace(/\s+/g, "-")}`;
          return (
            <section key={section.title} aria-labelledby={id} className={sectionClass}>
              <h2 id={id} className={headingClass}>
                {section.title}
              </h2>
              {i === 0 && (
                <p className="text-sm text-muted-foreground">
                  Fill in only the weeks when an activity happened. Leave a week blank when there was no activity; enter 0 when the activity happened but the count was zero.
                </p>
              )}
              {/* table-fixed gives every section the same column widths; min-w keeps phones scrolling sideways */}
              <Table aria-label={`${section.title} metrics`} className="min-w-[40rem] table-fixed">
                <TableHeader>
                  <TableRow>
                    <TableHead className="sticky left-0 w-56 bg-background">Metric</TableHead>
                    {WEEKS.map((w) => (
                      <TableHead key={w}>Week {w.slice(4)}</TableHead>
                    ))}
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {section.metrics.map((m) => (
                    <MetricRow
                      key={m.key}
                      metric={m}
                      values={form.metrics[m.key]}
                      errors={errors}
                      restricted={restricted}
                      onChange={setWeek}
                      onBlur={checkWeek}
                    />
                  ))}
                </TableBody>
              </Table>
            </section>
          );
        })}

        <section aria-labelledby="section-details" className={sectionClass}>
          <h2 id="section-details" className={headingClass}>
            Report Details
          </h2>
          {longTexts.map(([name, label, placeholder, help]) =>
            restricted ? (
              <div key={name} className="flex flex-col gap-2">
                <p className="text-sm font-medium">{label}</p>
                <p className="text-sm text-muted-foreground">Restricted</p>
              </div>
            ) : (
            <div key={name} className="flex flex-col gap-2">
              <Label htmlFor={name}>{label}</Label>
              <Textarea
                id={name}
                name={name}
                rows={4}
                placeholder={placeholder}
                value={form[name]}
                onChange={(e) => setField(name, e.target.value)}
                aria-invalid={!!errors[name]}
                aria-describedby={errors[name] ? `${name}-note ${name}-hint` : `${name}-hint`}
              />
              <p id={`${name}-hint`} className="text-sm text-muted-foreground">
                {help}
              </p>
              {errors[name] && (
                <p id={`${name}-note`} className="text-sm text-destructive">
                  {errors[name]}
                </p>
              )}
              <p className="text-xs text-muted-foreground">
                {[...form[name]].length} / {TEXT_LIMITS[name]}
              </p>
            </div>
            ),
          )}
        </section>
      </fieldset>

      {general && (
        <p role="alert" className="text-sm text-destructive">
          {general}
          {wait > 0 && ` Try again in ${wait}s.`}
        </p>
      )}
      {recovered && (
        <p role="status" className="text-sm text-muted-foreground">
          Recovered unsaved changes. <button type="button" className="font-medium text-foreground underline underline-offset-4" onClick={discardRecovery}>Discard</button>
        </p>
      )}
      {lastAutoSaved && (
        <p role="status" className="text-xs text-muted-foreground">
          Auto-saved at {lastAutoSaved.toLocaleTimeString([], { hour: "numeric", minute: "2-digit" })}
        </p>
      )}
      <div className="sticky bottom-0 z-10 -mx-4 flex flex-col gap-2 border-t bg-background/95 p-4 pb-[calc(1rem+env(safe-area-inset-bottom))] backdrop-blur sm:-mx-6 sm:flex-row sm:items-center sm:px-6">
        <div className="flex flex-col gap-2 sm:flex-row">
          {!readOnly && (
            <Button type="submit" value="submit" disabled={pending || wait > 0} aria-busy={pending}>
              {pending ? <><Loader2 aria-hidden="true" className="motion-safe:animate-spin" />Submitting…</> : draft ? "Submit Report" : revision ? "Resubmit Report" : submitLabel}
            </Button>
          )}
          <Button type="button" variant="ghost" onClick={cancel} disabled={pending}>
            {readOnly ? "Back" : "Cancel"}
          </Button>
        </div>
        {endAction && <div className="sm:ml-auto">{endAction}</div>}
      </div>
    </form>
  );
}

// Memoised so typing in one week re-renders only its row: setWeek keeps every other row's values object.
const MetricRow = memo(function MetricRow({
  metric: m,
  values,
  errors,
  restricted,
  onChange,
  onBlur,
}: {
  metric: (typeof METRICS)[number];
  values: FormState["metrics"][string] | undefined;
  errors: Record<string, string>;
  restricted: boolean;
  onChange: (metric: string, week: (typeof WEEKS)[number], value: string) => void;
  onBlur: (metric: string, week: (typeof WEEKS)[number], value: string) => void;
}) {
  return (
    <TableRow>
      <TableCell className="sticky left-0 bg-background font-medium">
        {m.label}
        {errors[`metrics.${m.key}`] && (
          <span className="block text-sm font-normal text-destructive">{errors[`metrics.${m.key}`]}</span>
        )}
      </TableCell>
      {restricted && m.key === RESTRICTED_METRIC ? (
        <TableCell colSpan={WEEKS.length} className="text-muted-foreground">
          Restricted
        </TableCell>
      ) : (
        WEEKS.map((w) => (
          <TableCell key={w}>
            <input
              inputMode={m.key === "tithes_offering" ? "decimal" : "numeric"}
              pattern={m.key === "tithes_offering" ? undefined : "[0-9]*"} // a hint only: the form has noValidate
              aria-label={`${m.label}, week ${w.slice(4)}`}
              aria-invalid={!!errors[`metrics.${m.key}.${w}`]}
              aria-describedby={errors[`metrics.${m.key}.${w}`] ? `err-${m.key}-${w}` : undefined}
              value={values?.[w] ?? ""}
              onChange={(e) => onChange(m.key, w, e.target.value)}
              onFocus={(e) => e.target.select()}
              onBlur={(e) => onBlur(m.key, w, e.target.value)}
              className="min-h-11 w-full min-w-16 rounded-sm border border-input bg-background px-2 text-right font-mono text-base tabular-nums outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 aria-invalid:border-destructive md:text-sm"
            />
            {errors[`metrics.${m.key}.${w}`] && (
              <span id={`err-${m.key}-${w}`} className="mt-1 block text-xs text-destructive">{errors[`metrics.${m.key}.${w}`]}</span>
            )}
          </TableCell>
        ))
      )}
    </TableRow>
  );
});
