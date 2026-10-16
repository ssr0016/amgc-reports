import { describe, expect, it } from "vitest";
import {
  emptyForm,
  fromReport,
  fromServerDetails,
  isRestricted,
  metricError,
  metricErrorKey,
  METRICS,
  TEXT_LIMITS,
  toInput,
  validate,
} from "./report-form";
import type { Report } from "./api-schema";

const period = { month: "April", year: 2026 };
const valid = () => ({ ...emptyForm(), month: "April", year: "2026" });

describe("metricError", () => {
  it("treats blank as not reported", () => {
    expect(metricError("outreach", "")).toBeUndefined();
    expect(metricError("outreach", "   ")).toBeUndefined();
  });

  it("wants whole numbers for counts", () => {
    for (const ok of ["0", "12", "9999999999"]) expect(metricError("outreach", ok)).toBeUndefined();
    for (const bad of ["1.5", "-1", "abc", "1e3", "12345678901"]) {
      expect(metricError("outreach", bad), bad).toBe("Must be a whole number");
    }
  });

  it("wants at most two decimals for tithes_offering", () => {
    for (const ok of ["0", "10", "10.5", "10.55"]) expect(metricError("tithes_offering", ok)).toBeUndefined();
    for (const bad of ["10.555", ".5", "10.", "-3", "1,000"]) {
      expect(metricError("tithes_offering", bad), bad).toBe("Must be a valid amount (0.00)");
    }
  });
});

describe("validate", () => {
  it("accepts a clean form", () => {
    expect(validate(valid(), period)).toEqual({});
  });

  it("rejects a month that is not a month name", () => {
    expect(validate({ ...valid(), month: "Smarch" }).month).toBe("Choose a month.");
  });

  it("pins the previous month when a period is given", () => {
    const msg = "Reports can only be created for the previous month.";
    expect(validate({ ...valid(), month: "May" }, period).month).toBe(msg);
    expect(validate({ ...valid(), year: "2025" }, period).month).toBe(msg);
    expect(validate({ ...valid(), month: "May" }).month).toBeUndefined();
  });

  it("limits the year to 2000-2100 and lets it be blank", () => {
    for (const y of ["1999", "2101", "26", "abcd"]) expect(validate({ ...valid(), year: y }).year).toBeDefined();
    expect(validate({ ...valid(), year: "" }).year).toBeUndefined();
  });

  it("counts characters, not UTF-16 units, against the text limits", () => {
    const emoji = "😀";
    expect(validate({ ...valid(), worker: emoji.repeat(TEXT_LIMITS.worker) }).worker).toBeUndefined();
    expect(validate({ ...valid(), worker: emoji.repeat(TEXT_LIMITS.worker + 1) }).worker).toBe(
      `Must be at most ${TEXT_LIMITS.worker} characters.`,
    );
  });

  it("keys metric errors by metric and week", () => {
    const f = { ...valid(), metrics: { outreach: { week2: "x" }, tithes_offering: { week1: "1.234" } } };
    expect(Object.keys(validate(f))).toEqual([
      metricErrorKey("outreach", "week2"),
      metricErrorKey("tithes_offering", "week1"),
    ]);
  });
});

describe("toInput", () => {
  it("sends only the weeks with a value and keeps zero", () => {
    const f = { ...valid(), metrics: { outreach: { week1: "0", week2: " 3 ", week3: "", week4: "  " }, training: {} } };
    expect(toInput(f).metrics).toEqual({ outreach: { week1: 0, week2: 3 } });
  });

  it("sends every text field (PUT replaces) and leaves a blank year out", () => {
    const f = { ...valid(), year: " ", worker: "Ana" };
    const out = toInput(f);
    expect(out.year).toBeUndefined();
    expect(out).toMatchObject({ month: "April", worker: "Ana", area_assignment: "", names: "", challenges: "" });
  });
});

describe("fromReport", () => {
  const report = {
    id: 1, created_by: 1, month: "March", year: 2026, completed: false, created_at: "", updated_at: "",
    worker: "Ana", metrics: { outreach: { week1: 2, week3: 0 } },
  } as Report;

  it("turns numbers into strings and missing text into empty strings", () => {
    const f = fromReport(report);
    expect(f).toMatchObject({ month: "March", year: "2026", worker: "Ana", names: "", church_name: "" });
    expect(f.metrics).toEqual({ outreach: { week1: "2", week3: "0" } });
  });

  it("round-trips through toInput", () => {
    expect(toInput(fromReport(report)).metrics).toEqual(report.metrics);
  });
});

describe("emptyForm", () => {
  it("prefills the profile fields and starts on a month name", () => {
    const f = emptyForm({ worker: "Ana", area_assignment: "North" });
    expect(f).toMatchObject({ worker: "Ana", area_assignment: "North", church_name: "" });
    expect(f.month).not.toBe("");
  });

  it("covers all 21 metrics", () => {
    expect(METRICS).toHaveLength(21);
  });
});

describe("fromServerDetails", () => {
  it("maps known fields and metrics, and folds the rest into general", () => {
    const out = fromServerDetails({
      month: "bad month",
      "metrics[outreach]": "bad metric",
      week3: "bad week",
      "metrics[nope]": "unknown metric",
      other: "odd",
    });
    expect(out.fields).toEqual({ month: "bad month", "metrics.outreach": "bad metric" });
    expect(out.general).toBe("Week 3: bad week metrics[nope]: unknown metric other: odd");
  });
});

// D-18: the backend (ReportService.redact) applies the same rule; keep the two in sync.
describe("isRestricted", () => {
  const report = { created_by: 7 };
  const perms = (...held: string[]) => (p: string) => held.includes(p);

  it("is false for the owner, and for write_any or delete_any", () => {
    expect(isRestricted(report, 7, perms())).toBe(false);
    expect(isRestricted(report, 8, perms("reports:write_any"))).toBe(false);
    expect(isRestricted(report, 8, perms("reports:delete_any"))).toBe(false);
  });

  it("is true for a reader with only read_any, and with no session user", () => {
    expect(isRestricted(report, 8, perms("reports:read", "reports:read_any"))).toBe(true);
    expect(isRestricted(report, undefined, perms())).toBe(true);
  });
});
