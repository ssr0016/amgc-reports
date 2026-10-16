"use client";

import { keepPreviousData, useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, useEffect, useRef, useState } from "react";
import { AuthField } from "@/components/auth-field";
import { Can } from "@/components/can";
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { NativeSelect } from "@/components/ui/native-select";
import { Label } from "@/components/ui/label";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { ApiError } from "@/lib/api";
import { retryText } from "@/lib/auth-form";
import {
  defaultPeriod,
  deleteReport,
  FIRST_YEAR,
  listReports,
  MONTHS,
  type Report,
  type ReportFilters,
} from "@/lib/reports";
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

// Which report a row action is for, so a screen reader user can tell the rows apart.
const rowName = (r: Report) => `${r.month} ${r.year}${r.worker ? `, ${r.worker}` : ""}`;

function ReportsList() {
  const router = useRouter();
  const params = useSearchParams();
  const { can, user } = useSession();
  const filters = filtersFrom(params);
  const allowed = can("reports:read");
  const thisYear = new Date().getFullYear();
  const years = Array.from({ length: thisYear - FIRST_YEAR + 1 }, (_, i) => FIRST_YEAR + i);

  const { data, error, isPending } = useQuery({
    queryKey: ["reports", filters],
    queryFn: () => listReports(filters),
    enabled: allowed,
    placeholderData: keepPreviousData,
    // Fresh for 30 s, so switching tabs back and forth does not refetch every time; the read limit is 60 a minute.
    staleTime: 30_000,
    refetchOnWindowFocus: true,
  });

  // The report the delete dialog is asking about, if any.
  const queryClient = useQueryClient();
  const [target, setTarget] = useState<{ id: number; period: string } | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState("");
  // The dialog is controlled and has no trigger element, so Radix cannot return focus
  // by itself: remember the Delete button that opened it (WCAG 2.4.3).
  const opener = useRef<HTMLElement | null>(null);

  async function confirmDelete() {
    if (!target) return;
    setDeleting(true);
    setDeleteError("");
    try {
      await deleteReport(target.id);
    } catch (err) {
      setDeleting(false);
      // 404: it was already deleted elsewhere, so the list below is just stale.
      if (!(err instanceof ApiError && err.status === 404)) {
        setDeleteError(
          err instanceof ApiError
            ? retryText(err)
            : "Could not delete the report. Try again.",
        );
        return;
      }
    }
    setDeleting(false);
    setTarget(null);
    // Deleting the last row of a later page would leave an empty page behind.
    const lastOnPage = (filters.page ?? 1) > 1 && data?.data.length === 1;
    await queryClient.invalidateQueries({ queryKey: ["reports"] });
    if (lastOnPage) goToPage((filters.page ?? 1) - 1);
  }

  // Another user's report is readable (read_any) but only its owner, or someone with the _any permission, may change it.
  const mine = (r: Report) => r.created_by === user?.user_id;
  const canEdit = (r: Report) => can("reports:write") && (mine(r) || can("reports:write_any"));
  const canDelete = (r: Report) => can("reports:delete") && (mine(r) || can("reports:delete_any"));

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
            autoComplete="off"
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
        <p role="status" className="text-sm text-muted-foreground">Loading…</p>
      ) : error ? (
        <p role="alert" className="text-sm text-destructive">
          {error.message}
        </p>
      ) : data && data.data.length === 0 ? (
        <p role="status" className="text-sm text-muted-foreground">No reports match these filters.</p>
      ) : (
        data && (
          <>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Worker</TableHead>
                  <TableHead>Month</TableHead>
                  <TableHead>Year</TableHead>
                  <TableHead>Area of Assignment</TableHead>
                  <TableHead>Church</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {data.data.map((r) => (
                  <TableRow key={r.id}>
                    <TableCell>{r.worker || "—"}</TableCell>
                    <TableCell>{r.month}</TableCell>
                    <TableCell>{r.year}</TableCell>
                    <TableCell>{r.area_assignment || "—"}</TableCell>
                    <TableCell>{r.church_name || "—"}</TableCell>
                    <TableCell>
                      <Badge variant={r.completed ? "default" : "secondary"}>
                        {r.completed ? "Completed" : "Pending"}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <div className="flex gap-2">
                        <Button asChild variant="outline" size="sm">
                          <Link href={`/reports/${r.id}/edit`}>{canEdit(r) ? "Edit" : "View"}{" "}
                            <span className="sr-only">{rowName(r)}</span>
                          </Link>
                        </Button>
                        {canDelete(r) && (
                          <Button
                            variant="destructive"
                            size="sm"
                            onClick={(e) => {
                              opener.current = e.currentTarget;
                              setDeleteError("");
                              setTarget({ id: r.id, period: `${r.month} ${r.year}` });
                            }}
                          >
                            Delete{" "}
                            <span className="sr-only">{rowName(r)}</span>
                          </Button>
                        )}
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
            {meta && (
              <div className="flex items-center justify-between gap-4">
                <p role="status" className="text-sm text-muted-foreground">
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

      <AlertDialog open={target !== null} onOpenChange={(open) => !open && !deleting && setTarget(null)}>
        <AlertDialogContent
          onCloseAutoFocus={(e) => {
            e.preventDefault();
            // After a delete the row is gone; the main content is the next best place.
            (opener.current?.isConnected ? opener.current : document.getElementById("main-content"))?.focus();
          }}
        >
          <AlertDialogHeader>
            <AlertDialogTitle>Delete {target?.period}?</AlertDialogTitle>
            <AlertDialogDescription>You can file this month again afterwards.</AlertDialogDescription>
          </AlertDialogHeader>
          {deleteError && (
            <p role="alert" className="text-sm text-destructive">
              {deleteError}
            </p>
          )}
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleting}>Cancel</AlertDialogCancel>
            {/* A plain Button, not AlertDialogAction: that one closes the dialog on click, and a failed delete must keep it open. */}
            <Button variant="destructive" onClick={confirmDelete} disabled={deleting}>
              {deleting ? "Deleting…" : "Delete"}
            </Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}

export function Reports() {
  return (
    <main id="main-content" tabIndex={-1} className="mx-auto flex w-full max-w-5xl flex-1 flex-col gap-6 p-4 focus:outline-none">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold tracking-tight">Reports</h1>
        <Can permission="reports:write">
          <Button asChild>
            <Link href="/reports/new">New report</Link>
          </Button>
        </Can>
      </div>
      {/* useSearchParams needs a Suspense boundary for the static prerender */}
      <Suspense fallback={<p role="status" className="text-sm text-muted-foreground">Loading…</p>}>
        <ReportsList />
      </Suspense>
    </main>
  );
}
