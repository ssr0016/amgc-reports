import type { Permission } from "@/lib/permissions";
import { defaultPeriod, MONTHS, type Report, type ReportInput } from "./reports";

// Labels from the legacy Express field names (backend/docs/05-Backend-Schema.md 2.4).
export const METRICS = [
  { key: "worship_service", label: "Worship Service" },
  { key: "sunday_school", label: "Sunday School" },
  { key: "prayer_meeting", label: "Prayer Meeting" },
  { key: "bible_studies", label: "Bible Studies" },
  { key: "mens_fellowship", label: "Men's Fellowship" },
  { key: "womens_fellowship", label: "Women's Fellowship" },
  { key: "youth_fellowship", label: "Youth Fellowship" },
  { key: "children_fellowship", label: "Children's Fellowship" },
  { key: "tithes_offering", label: "Tithes & Offering" },
  { key: "home_visited", label: "Homes Visited" },
  { key: "bible_study_group_led", label: "Bible Study Group Led" },
  { key: "sermon_preached", label: "Sermons Preached" },
  { key: "person_newly_contacted", label: "Newly Contacted" },
  { key: "person_followed_up", label: "Persons Followed Up" },
  { key: "person_led_to_christ", label: "Persons Led to Christ" },
  { key: "outreach", label: "Outreach" },
  { key: "training", label: "Training" },
  { key: "leadership", label: "Leadership" },
  { key: "baptism", label: "Baptism" },
  { key: "other", label: "Other" },
  { key: "family_day", label: "Family Day" },
  // The old field, kept for historical values whose meaning is unverified (D-A31).
  // The form shows it only on a report that already has it.
  { key: "person_evangelized", label: "Legacy Persons Evangelized — meaning unverified" },
] as const;

export const LEGACY_METRIC = "person_evangelized";

type MetricKey = (typeof METRICS)[number]["key"];

// How the form groups the 21 metrics. Every key appears exactly once. The
// owner's list of sections left out three metrics: Youth and Children's
// Fellowship join Fellowship, and Tithes & Offering, a weekly figure taken at
// the services, joins Weekly Attendance. Move a key here to change that.
const SECTIONS: { title: string; keys: MetricKey[] }[] = [
  { title: "Weekly Attendance", keys: ["worship_service", "sunday_school", "prayer_meeting", "bible_studies", "tithes_offering"] },
  { title: "Fellowship", keys: ["mens_fellowship", "womens_fellowship", "youth_fellowship", "children_fellowship"] },
  {
    title: "Weekly Ministry",
    keys: ["home_visited", "bible_study_group_led", "sermon_preached", "person_newly_contacted", "person_followed_up", "person_led_to_christ", "person_evangelized"],
  },
  { title: "Ministry Activities", keys: ["outreach", "training", "leadership", "baptism", "other", "family_day"] },
];

export const METRIC_SECTIONS = SECTIONS.map((section) => ({
  title: section.title,
  metrics: section.keys.map((key) => METRICS.find((m) => m.key === key)!),
}));

export const WEEKS = ["week1", "week2", "week3", "week4", "week5"] as const;
type Week = (typeof WEEKS)[number];

export const TEXT_LIMITS = {
  worker: 150,
  area_assignment: 150,
  church_name: 150,
  names: 5000,
  narrative_report: 10000,
  challenges: 10000,
  prayer_request: 10000,
} as const;
type TextField = keyof typeof TEXT_LIMITS;

// Everything is a string while the user types, so "0.50" and "" stay distinct:
// blank means not reported, "0" means reported zero.
export type FormState = Record<TextField, string> & {
  month: string;
  year: string;
  metrics: Record<string, Partial<Record<Week, string>>>;
};

export const metricErrorKey = (metric: string, week: Week) => `metrics.${metric}.${week}`;

// A new report starts on the previous month, the only one it may be filed for.
// `defaults` comes from the user: worker is their name, area and church their profile.
// The user may edit all three.
export function emptyForm(
  defaults: Partial<Pick<FormState, "worker" | "area_assignment" | "church_name">> = {},
): FormState {
  const { month, year } = defaultPeriod();
  return {
    month,
    year: String(year),
    worker: defaults.worker ?? "",
    area_assignment: defaults.area_assignment ?? "",
    church_name: defaults.church_name ?? "",
    names: "",
    narrative_report: "",
    challenges: "",
    prayer_request: "",
    metrics: {},
  };
}

// The API leaves out empty text, so a missing field is "".
export function fromReport(r: Report): FormState {
  const metrics: FormState["metrics"] = {};
  for (const [metric, weeks] of Object.entries(r.metrics)) {
    metrics[metric] = {};
    for (const week of WEEKS) {
      const value = weeks[week];
      if (value !== undefined) metrics[metric][week] = String(value);
    }
  }
  return {
    month: r.month,
    year: String(r.year),
    worker: r.worker ?? "",
    area_assignment: r.area_assignment ?? "",
    church_name: r.church_name ?? "",
    names: r.names ?? "",
    narrative_report: r.narrative_report ?? "",
    challenges: r.challenges ?? "",
    prayer_request: r.prayer_request ?? "",
    metrics,
  };
}

// Sends every text field (PUT is a full replace) and only the weeks with a value.
// Run validate() first: Number() of a valid 2-decimal value below 1e10 is exact.
export function toInput(f: FormState): ReportInput {
  const metrics: ReportInput["metrics"] = {};
  for (const [metric, weeks] of Object.entries(f.metrics)) {
    const sent: Partial<Record<Week, number>> = {};
    for (const week of WEEKS) {
      const text = weeks[week]?.trim();
      if (text) sent[week] = Number(text);
    }
    if (Object.keys(sent).length > 0) metrics[metric] = sent;
  }
  return {
    month: f.month as ReportInput["month"], // validate() has checked it is a month name
    year: f.year.trim() ? Number(f.year) : undefined,
    worker: f.worker,
    area_assignment: f.area_assignment,
    church_name: f.church_name,
    names: f.names,
    narrative_report: f.narrative_report,
    challenges: f.challenges,
    prayer_request: f.prayer_request,
    metrics,
  };
}

const DECIMAL = /^\d{1,10}(\.\d{1,2})?$/;
const WHOLE = /^\d{1,10}$/;

// One rule for blur and submit. Only tithes_offering is money; the other 20 count
// people or events. Whole numbers are a client rule: the server accepts decimals for all 21.
export function metricError(metric: string, text: string): string | undefined {
  const t = text.trim();
  if (!t) return undefined; // blank means not reported
  if (metric === "tithes_offering") return DECIMAL.test(t) ? undefined : "Must be a valid amount (0.00)";
  return WHOLE.test(t) ? undefined : "Must be a whole number";
}

// Mirrors the server rules for quick feedback; the server stays the authority.
// Returns a map of field name to message; empty means valid. `period` pins the
// month and year (a new report may only be filed for the previous month).
export function validate(f: FormState, period?: { month: string; year: number }): Record<string, string> {
  const errors: Record<string, string> = {};
  if (!(MONTHS as readonly string[]).includes(f.month)) errors.month = "Choose a month.";
  else if (period && (f.month !== period.month || Number(f.year) !== period.year)) {
    errors.month = "Reports can only be created for the previous month.";
  }
  const year = f.year.trim();
  if (year && !(/^\d{4}$/.test(year) && Number(year) >= 2000 && Number(year) <= 2100)) {
    errors.year = "Enter a year from 2000 to 2100.";
  }
  for (const [field, max] of Object.entries(TEXT_LIMITS)) {
    // Spread counts characters, as the server does, not UTF-16 units.
    if ([...f[field as TextField]].length > max) errors[field] = `Must be at most ${max} characters.`;
  }
  for (const [metric, weeks] of Object.entries(f.metrics)) {
    for (const week of WEEKS) {
      const message = metricError(metric, weeks[week] ?? "");
      if (message) errors[metricErrorKey(metric, week)] = message;
    }
  }
  return errors;
}

// Maps the 422 `details` of the server into the shape validate() returns.
// The server names a bad metric as "metrics[key]" but reports a bad week value
// as a bare "week1".."week5" with no metric, so those (and anything else the
// form has no field for) go to `general`.
export function fromServerDetails(details: Record<string, string>): {
  fields: Record<string, string>;
  general: string;
} {
  const fields: Record<string, string> = {};
  const general: string[] = [];
  const known = new Set<string>(["month", "year", ...Object.keys(TEXT_LIMITS)]);
  for (const [key, message] of Object.entries(details)) {
    const metric = /^metrics\[(.+)\]$/.exec(key)?.[1];
    if (metric && METRICS.some((m) => m.key === metric)) fields[`metrics.${metric}`] = message;
    else if (known.has(key)) fields[key] = message;
    else if (/^week[1-5]$/.test(key)) general.push(`Week ${key.slice(4)}: ${message}`);
    else general.push(`${key}: ${message}`);
  }
  return { fields, general: general.join(" ") };
}

// D-18: free text and tithes_offering are hidden from a reader who neither owns the report nor holds
// write_any or delete_any. The server removes them (ReportService.redact); this only tells the form why
// they are missing. Keep the two rules in sync.
export const RESTRICTED_METRIC = "tithes_offering";

export const isRestricted = (report: { created_by: number }, userId: number | undefined, can: (p: Permission) => boolean) =>
  report.created_by !== userId && !can("reports:write_any") && !can("reports:delete_any");
