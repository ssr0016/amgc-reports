"use client";

import { keepPreviousData, useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense } from "react";
import { AuthField } from "@/components/auth-field";
import { Can } from "@/components/can";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { NativeSelect } from "@/components/ui/native-select";
import { Label } from "@/components/ui/label";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { ApiError } from "@/lib/api";
import { defaultPeriod, listReports, MONTHS, type ReportFilters } from "@/lib/reports";
import { useSession } from "@/lib/session";

const textFields = [
  ["worker", "e.g. Juan Dela Cruz"],
  ["area", "e.g. Metro Manila"],
  ["church", "e.g. AMGC Church"],
] as const;

// A month or year missing from the URL means the default period; present but
// empty ("?month=&year=") means any. Clear and an "Any" submit write the empty form.
function filtersFrom(params: URLSearchParams): ReportFilters {
  const def = defaultPeriod();
  const year = params.has("year") ? Number(params.get("year")) : def.year;
  const page = Number(params.get("page"));
  return {
    month: (params.has("month") ? params.get("month") : def.month) || undefined,
    year: year > 0 ? year : undefined,
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
  const hasFilters = Object.entries(filters).some(([key, value]) => key !== "page" && value !== undefined);

  const { data, error, isPending } = useQuery({
    queryKey: ["reports", filters],
    queryFn: () => listReports(filters),
    enabled: allowed,
    placeholderData: keepPreviousData,
  });

  // Filters apply on submit, not per keystroke, to spare the read rate limit.
  function onSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const next = new URLSearchParams();
    for (const [key, value] of new FormData(e.currentTarget)) {
      // month and year are always written, even empty, so "any" survives a reload
      if (String(value).trim() || key === "month" || key === "year") next.set(key, String(value).trim());
    }
    router.push(`/reports?${next}`);
  }

  function goToPage(page: number) {
    const next = new URLSearchParams(params);
    next.set("page", String(page));
    router.push(`/reports?${next}`);
  }

  if (!allowed || (error instanceof ApiError && error.status === 403)) {
    return <p className="text-sm text-muted-foreground">You are not allowed to view reports.</p>;
  }

  const meta = data?.meta;
  return (
    <>
      {/* key remounts the form so Clear and Back reset the inputs */}
      <form key={params.toString()} onSubmit={onSubmit} className="grid gap-4 sm:grid-cols-2 lg:grid-cols-5">
        <div className="flex flex-col gap-2">
          <Label htmlFor="month">Month</Label>
          <NativeSelect id="month" name="month" defaultValue={filters.month ?? ""}>
            <option value="">Any</option>
            {MONTHS.map((m) => (
              <option key={m} value={m}>
                {m}
              </option>
            ))}
          </NativeSelect>
        </div>
        <AuthField name="year" label="Year" type="number" min={2000} max={2100} defaultValue={filters.year ?? ""} />
        {textFields.map(([name, placeholder]) => (
          <AuthField
            key={name}
            name={name}
            label={name[0].toUpperCase() + name.slice(1)}
            placeholder={placeholder}
            defaultValue={filters[name] ?? ""}
          />
        ))}
        <div className="flex gap-2 sm:col-span-2 lg:col-span-5">
          <Button type="submit">Apply filters</Button>
          <Button type="button" variant="outline" onClick={() => router.push("/reports?month=&year=")}>
            Clear
          </Button>
        </div>
      </form>

      {isPending && allowed ? (
        <p className="text-sm text-muted-foreground">Loading…</p>
      ) : error ? (
        <p role="alert" className="text-sm text-destructive">
          {error.message}
        </p>
      ) : data && data.data.length === 0 ? (
        <p className="text-sm text-muted-foreground">
          {hasFilters ? "No reports match these filters." : "No reports yet."}
        </p>
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
        <div className="flex items-center gap-4">
          <Can permission="reports:write">
            <Button asChild>
              <Link href="/reports/new">New report</Link>
            </Button>
          </Can>
          <Link href="/" className="text-sm underline underline-offset-4">
            Home
          </Link>
        </div>
      </div>
      {/* useSearchParams needs a Suspense boundary for the static prerender */}
      <Suspense fallback={<p className="text-sm text-muted-foreground">Loading…</p>}>
        <ReportsList />
      </Suspense>
    </main>
  );
}
