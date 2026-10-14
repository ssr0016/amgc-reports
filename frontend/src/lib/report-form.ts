import { MONTHS, type Report, type ReportInput } from "./reports";

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
  { key: "bible_study_group_led", label: "Bible Study Groups Led" },
  { key: "sermon_preached", label: "Sermons Preached" },
  { key: "person_newly_contacted", label: "Persons Newly Contacted" },
  { key: "person_followed_up", label: "Persons Followed Up" },
  { key: "person_evangelized", label: "Persons Evangelized" },
  { key: "outreach", label: "Outreach" },
  { key: "training", label: "Training" },
  { key: "leadership", label: "Leadership" },
  { key: "baptism", label: "Baptism" },
  { key: "other", label: "Other" },
  { key: "family_day", label: "Family Day" },
] as const;

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

export function emptyForm(): FormState {
  return {
    month: "",
    year: String(new Date().getFullYear()),
    worker: "",
    area_assignment: "",
    church_name: "",
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

// Mirrors the server rules for quick feedback; the server stays the authority.
// Returns a map of field name to message; empty means valid.
export function validate(f: FormState): Record<string, string> {
  const errors: Record<string, string> = {};
  if (!(MONTHS as readonly string[]).includes(f.month)) errors.month = "Choose a month.";
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
      const text = weeks[week]?.trim();
      if (text && !DECIMAL.test(text)) {
        errors[metricErrorKey(metric, week)] = "Use a number from 0 with at most 2 decimals.";
      }
    }
  }
  return errors;
}
