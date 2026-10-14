"use client";

import { useEffect, useState } from "react";
import { AuthField } from "@/components/auth-field";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { NativeSelect } from "@/components/ui/native-select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Textarea } from "@/components/ui/textarea";
import { ApiError } from "@/lib/api";
import { toFormError } from "@/lib/auth-form";
import {
  fromServerDetails,
  METRIC_SECTIONS,
  metricError,
  metricErrorKey,
  METRICS,
  TEXT_LIMITS,
  toInput,
  validate,
  WEEKS,
  type FormState,
} from "@/lib/report-form";
import { MONTHS, type ReportInput } from "@/lib/reports";

const shortTexts = [
  ["worker", "Worker Name", "e.g. Juan Dela Cruz"],
  ["area_assignment", "Area Assignment", "e.g. Metro Manila"],
  ["church_name", "Church Name", "e.g. AMGC Church"],
] as const;

const longTexts = [
  ["names", "Names of New Believers", "e.g. Juan Dela Cruz, Maria Santos"],
  ["narrative_report", "Narrative Report", "Summarize the month's activities..."],
  ["challenges", "Challenges", "Describe any challenges..."],
  ["prayer_request", "Prayer Request", "List prayer requests..."],
] as const;

const sectionClass = "flex flex-col gap-4";
const headingClass = "text-lg font-medium";

// A failed save leaves the form as it is, so nothing the user typed is lost.
export function ReportForm({
  initial,
  submitLabel,
  lockedPeriod,
  onSubmit,
  onCancel,
}: {
  initial: FormState;
  submitLabel: string;
  // When set, month and year cannot be changed and must match it.
  lockedPeriod?: { month: string; year: number };
  onSubmit: (input: ReportInput) => Promise<void>;
  onCancel: () => void;
}) {
  const [start] = useState(() => JSON.stringify(initial));
  const [form, setForm] = useState(initial);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [general, setGeneral] = useState("");
  const [pending, setPending] = useState(false);
  const [saved, setSaved] = useState(false);
  const dirty = !saved && JSON.stringify(form) !== start;

  // Covers closing the tab or refreshing; in-app navigation is guarded by Cancel.
  useEffect(() => {
    if (!dirty) return;
    const warn = (e: BeforeUnloadEvent) => e.preventDefault();
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, [dirty]);

  function clearError(...keys: string[]) {
    setErrors((prev) => Object.fromEntries(Object.entries(prev).filter(([k]) => !keys.includes(k))));
  }

  function setField(name: keyof Omit<FormState, "metrics">, value: string) {
    setForm((f) => ({ ...f, [name]: value }));
    clearError(name);
  }

  function setWeek(metric: string, week: (typeof WEEKS)[number], value: string) {
    setForm((f) => ({ ...f, metrics: { ...f.metrics, [metric]: { ...f.metrics[metric], [week]: value } } }));
    clearError(`metrics.${metric}.${week}`, `metrics.${metric}`);
  }

  function checkWeek(metric: string, week: (typeof WEEKS)[number], value: string) {
    const key = metricErrorKey(metric, week);
    const message = metricError(metric, value);
    setErrors((prev) => {
      const next = { ...prev };
      if (message) next[key] = message;
      else delete next[key];
      return next;
    });
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    const found = validate(form, lockedPeriod);
    setErrors(found);
    setGeneral(Object.keys(found).length ? "Fix the highlighted fields and try again." : "");
    if (Object.keys(found).length) return;

    setPending(true);
    try {
      await onSubmit(toInput(form));
      setSaved(true);
    } catch (err) {
      setPending(false);
      if (err instanceof ApiError && err.isConflict) {
        setErrors({ month: "You already have a report for this month and year." });
        setGeneral("");
        return;
      }
      const failure = toFormError(err);
      const mapped = fromServerDetails(failure.fields);
      const wait = failure.retryAfter ? ` Try again in ${failure.retryAfter}s.` : "";
      setErrors(mapped.fields);
      setGeneral(failure.message ? failure.message + wait : mapped.general || "Fix the highlighted fields and try again.");
    }
  }

  function cancel() {
    if (!dirty || confirm("Discard your changes?")) onCancel();
  }

  const weekErrors = METRICS.flatMap((m) =>
    WEEKS.flatMap((w) => {
      const message = errors[`metrics.${m.key}.${w}`];
      return message ? [`${m.label}, week ${w.slice(4)}: ${message}`] : [];
    }),
  );

  return (
    <form onSubmit={submit} noValidate className="flex flex-col gap-8">
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
              <p id="month-note" className="text-sm text-destructive">
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
          {shortTexts.map(([name, label, placeholder]) => (
            <AuthField
              key={name}
              name={name}
              label={label}
              placeholder={placeholder}
              value={form[name]}
              onChange={(e) => setField(name, e.target.value)}
              error={errors[name]}
            />
          ))}
        </div>
      </section>

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
          <section key={section.title} aria-labelledby={id} className={`${sectionClass} border-t pt-6`}>
            <h2 id={id} className={headingClass}>
              {section.title}
            </h2>
            {i === 0 && (
              <p className="text-sm text-muted-foreground">
                Leave a week blank if it was not reported. Enter 0 if it was reported as zero.
              </p>
            )}
            {/* table-fixed gives every section the same column widths; min-w keeps phones scrolling sideways */}
            <Table className="min-w-[40rem] table-fixed">
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
                  <TableRow key={m.key}>
                    <TableCell className="sticky left-0 bg-background font-medium">
                      {m.label}
                      {errors[`metrics.${m.key}`] && (
                        <span className="block text-sm font-normal text-destructive">{errors[`metrics.${m.key}`]}</span>
                      )}
                    </TableCell>
                    {WEEKS.map((w) => (
                      <TableCell key={w}>
                        <input
                          inputMode={m.key === "tithes_offering" ? "decimal" : "numeric"}
                          pattern={m.key === "tithes_offering" ? undefined : "[0-9]*"} // a hint only: the form has noValidate
                          aria-label={`${m.label}, week ${w.slice(4)}`}
                          aria-invalid={!!errors[`metrics.${m.key}.${w}`]}
                          value={form.metrics[m.key]?.[w] ?? ""}
                          onChange={(e) => setWeek(m.key, w, e.target.value)}
                          onFocus={(e) => e.target.select()}
                          onBlur={(e) => checkWeek(m.key, w, e.target.value)}
                          className="h-8 w-20 rounded-lg border border-input bg-transparent px-2 text-base outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 aria-invalid:border-destructive md:text-sm dark:bg-input/30"
                        />
                        {errors[`metrics.${m.key}.${w}`] && (
                          <span className="mt-1 block text-xs text-destructive">{errors[`metrics.${m.key}.${w}`]}</span>
                        )}
                      </TableCell>
                    ))}
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </section>
        );
      })}

      <section aria-labelledby="section-details" className={`${sectionClass} border-t pt-6`}>
        <h2 id="section-details" className={headingClass}>
          Report Details
        </h2>
        {longTexts.map(([name, label, placeholder]) => (
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
              aria-describedby={errors[name] ? `${name}-note` : undefined}
            />
            {errors[name] && (
              <p id={`${name}-note`} className="text-sm text-destructive">
                {errors[name]}
              </p>
            )}
            <p className="text-xs text-muted-foreground">
              {[...form[name]].length} / {TEXT_LIMITS[name]}
            </p>
          </div>
        ))}
      </section>

      {general && (
        <p role="alert" className="text-sm text-destructive">
          {general}
        </p>
      )}
      <div className="flex gap-2">
        <Button type="submit" disabled={pending}>
          {pending ? "Saving…" : submitLabel}
        </Button>
        <Button type="button" variant="outline" onClick={cancel} disabled={pending}>
          Cancel
        </Button>
      </div>
    </form>
  );
}
