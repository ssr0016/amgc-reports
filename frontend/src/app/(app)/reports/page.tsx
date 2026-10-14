"use client";

import { keepPreviousData, useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, useEffect, useRef, useState } from "react";
import { AuthField } from "@/components/auth-field";
import { Can } from "@/components/can";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { NativeSelect } from "@/components/ui/native-select";
import { Label } from "@/components/ui/label";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { ApiError } from "@/lib/api";
import { defaultPeriod, FIRST_YEAR, listReports, MONTHS, type ReportFilters } from "@/lib/reports";
import { useSession } from "@/lib/session";

const textFields = [
  ["worker", "e.g. Juan Dela Cruz"],
  ["area", "e.g. Metro Manila"],
  ["church", "e.g. AMGC Church"],
] as const;
type Text = Record<(typeof textFields)[number][0], string>;

// A month missing from the URL means the default; present but empty ("month=")
// means any month. The year is always one of FIRST_YEAR..this year.
function filtersFrom(params: URLSearchParams): ReportFilters {
  const def = defaultPeriod();
  const year = Number(params.get("year"));
  const page = Number(params.get("page"));
  return {
    month: (params.has("month") ? params.get("month") : def.month) || undefined,
    year: Number.isInteger(year) && year >= FIRST_YEAR && year <= new Date().getFullYear() ? year : def.year,
    worker: params.get("worker") || undefined,
    area: params.get("area") || undefined,
    church: params.get("church") || undefined,
    page: page > 1 ? page : undefined,
  };
}

function ReportsList() {
  const router = useRouter();
  const params = useSearchParams();
  const { can } = useSession();
  const filters = filtersFrom(params);
  const allowed = can("reports:read");
  const thisYear = new Date().getFullYear();
  const years = Array.from({ length: thisYear - FIRST_YEAR + 1 }, (_, i) => FIRST_YEAR + i);

  const { data, error, isPending } = useQuery({
    queryKey: ["reports", filters],
    queryFn: () => listReports(filters),
    enabled: allowed,
    placeholderData: keepPreviousData,
  });

  // Filters apply as they change. The text boxes wait 300 ms after the last key,
  // so typing does not spend the per-user read rate limit one request per letter.
  const urlText: Text = { worker: filters.worker ?? "", area: filters.area ?? "", church: filters.church ?? "" };
  const [text, setText] = useState<Text>(urlText);
  const urlKey = JSON.stringify(urlText);
  const sentKey = useRef(urlKey);

  function go(next: { month?: string; year: number } & Text, mode: "push" | "replace") {
    const q = new URLSearchParams({ month: next.month ?? "", year: String(next.year) });
    for (const [name] of textFields) if (next[name].trim()) q.set(name, next[name].trim());
    router[mode](`/reports?${q}`);
  }

  useEffect(() => {
    const key = JSON.stringify(text);
    if (key === sentKey.current) return;
    const timer = setTimeout(() => {
      sentKey.current = key;
      go({ month: filters.month, year: filters.year!, ...text }, "replace");
    }, 300);
    return () => clearTimeout(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- go() only reads the values listed here
  }, [text, filters.month, filters.year]);

  // The URL changed from outside (Back, a shared link): show its text in the boxes.
  useEffect(() => {
    if (urlKey === sentKey.current) return;
    sentKey.current = urlKey;
    setText(JSON.parse(urlKey));
  }, [urlKey]);

  function goToPage(page: number) {
    const next = new URLSearchParams(params);
    next.set("page", String(page));
    router.push(`/reports?${next}`);
  }

  if (!allowed || (error instanceof ApiError && error.status === 403)) {
    return <p className="text-sm text-muted-foreground">You are not allowed to view reports.</p>;
  }

  const meta = data?.meta;
  const hasText = textFields.some(([name]) => text[name] !== "");
  return (
    <>
      <form onSubmit={(e) => e.preventDefault()} className="grid gap-4 sm:grid-cols-2 lg:grid-cols-5">
        <div className="flex flex-col gap-2">
          <Label htmlFor="month">Month</Label>
          <NativeSelect
            id="month"
            value={filters.month ?? ""}
            onChange={(e) => go({ month: e.target.value, year: filters.year!, ...text }, "push")}
          >
            <option value="">Any</option>
            {MONTHS.map((m) => (
              <option key={m} value={m}>
                {m}
              </option>
            ))}
          </NativeSelect>
        </div>
        <div className="flex flex-col gap-2">
          <Label htmlFor="year">Year</Label>
          <NativeSelect
            id="year"
            value={filters.year}
            onChange={(e) => go({ month: filters.month, year: Number(e.target.value), ...text }, "push")}
          >
            {years.map((y) => (
              <option key={y} value={y}>
                {y}
              </option>
            ))}
          </NativeSelect>
        </div>
        {textFields.map(([name, placeholder]) => (
          <AuthField
            key={name}
            name={name}
            label={name[0].toUpperCase() + name.slice(1)}
            placeholder={placeholder}
            value={text[name]}
            onChange={(e) => setText({ ...text, [name]: e.target.value })}
          />
        ))}
        {hasText && (
          <div className="sm:col-span-2 lg:col-span-5">
            <Button
              type="button"
              variant="outline"
              onClick={() => {
                const empty = { worker: "", area: "", church: "" };
                sentKey.current = JSON.stringify(empty);
                setText(empty);
                go({ month: filters.month, year: filters.year!, ...empty }, "push");
              }}
            >
              Clear
            </Button>
          </div>
        )}
      </form>

      {isPending && allowed ? (
        <p className="text-sm text-muted-foreground">Loading…</p>
      ) : error ? (
        <p role="alert" className="text-sm text-destructive">
          {error.message}
        </p>
      ) : data && data.data.length === 0 ? (
        <p className="text-sm text-muted-foreground">No reports match these filters.</p>
      ) : (
        data && (
          <>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Month</TableHead>
                  <TableHead>Year</TableHead>
                  <TableHead>Worker</TableHead>
                  <TableHead>Church</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>
                    <span className="sr-only">Actions</span>
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {data.data.map((r) => (
                  <TableRow key={r.id}>
                    <TableCell>{r.month}</TableCell>
                    <TableCell>{r.year}</TableCell>
                    <TableCell>{r.worker || "—"}</TableCell>
                    <TableCell>{r.church_name || "—"}</TableCell>
                    <TableCell>
                      <Badge variant={r.completed ? "default" : "secondary"}>
                        {r.completed ? "Completed" : "Pending"}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <Can permission="reports:write">
                        <Link href={`/reports/${r.id}/edit`} className="underline underline-offset-4">
                          Edit
                        </Link>
                      </Can>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
            {meta && (
              <div className="flex items-center justify-between gap-4">
                <p className="text-sm text-muted-foreground">
                  Page {meta.page} of {meta.total_pages} ({meta.total} reports)
                </p>
                <div className="flex gap-2">
                  <Button variant="outline" disabled={meta.page <= 1} onClick={() => goToPage(meta.page - 1)}>
                    Previous
                  </Button>
                  <Button
                    variant="outline"
                    disabled={meta.page >= meta.total_pages}
                    onClick={() => goToPage(meta.page + 1)}
                  >
                    Next
                  </Button>
                </div>
              </div>
            )}
          </>
        )
      )}
    </>
  );
}

export default function ReportsPage() {
  return (
    <main className="mx-auto flex w-full max-w-5xl flex-1 flex-col gap-6 p-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold tracking-tight">Reports</h1>
        <Can permission="reports:write">
          <Button asChild>
            <Link href="/reports/new">New report</Link>
          </Button>
        </Can>
      </div>
      {/* useSearchParams needs a Suspense boundary for the static prerender */}
      <Suspense fallback={<p className="text-sm text-muted-foreground">Loading…</p>}>
        <ReportsList />
      </Suspense>
    </main>
  );
}
