import { change } from "@/lib/analytics";
import type { AttendanceAnalytics } from "@/lib/api-schema";

const peso = new Intl.NumberFormat("en-PH", { style: "currency", currency: "PHP", minimumFractionDigits: 2, maximumFractionDigits: 2 });

// formatValue shows a number in its unit; null is "no data", never 0.
export function formatValue(value: number | null | undefined, unit?: string, empty = "—"): string {
  if (value == null) return empty;
  if (unit === "php") return peso.format(value);
  return value.toLocaleString(undefined, { maximumFractionDigits: 2 });
}

export function formatPct(ratio: number | null | undefined): string {
  if (ratio == null) return "n/a";
  return `${ratio >= 0 ? "+" : ""}${(ratio * 100).toLocaleString(undefined, { maximumFractionDigits: 1 })}%`;
}

// changeText names why a change is unavailable instead of showing 0% or "New".
export function changeText(current: number | null | undefined, previous: number | null | undefined, year: number): string {
  if (previous == null) return `vs ${year}: unavailable (no data)`;
  if (previous === 0) return `vs ${year}: unavailable (previous was 0)`;
  if (current == null) return `vs ${year}: unavailable (no data this period)`;
  return `vs ${year}: ${formatPct(change(current, previous))}`;
}

// missingText names why a church has no value for the selected activity. Under the
// Approved filter a report still in review is not missing: it shows its review status.
export function missingText(data: AttendanceAnalytics, churchId: number, empty = "No data"): string {
  const state = data.metrics.find((m) => m.metric === data.metric)?.churches.find((c) => c.church_id === churchId)?.state;
  if (state !== "not_approved") return empty;
  const comp = data.compliance.find((r) => r.church_id === churchId);
  return comp && comp.pending === 0 && comp.needs_revision > 0 ? "Needs revision" : "Pending approval";
}
