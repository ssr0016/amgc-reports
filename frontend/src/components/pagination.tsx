"use client";

import { useId } from "react";
import { ChevronLeft, ChevronRight } from "lucide-react";
import { Button } from "@/components/ui/button";
import { NativeSelect } from "@/components/ui/native-select";

export const PAGE_SIZES = [10, 25, 50] as const;

// Hover lift to match the admin buttons; the shared Button only transitions colours.
const lift = "min-h-11 transition-[color,background-color,box-shadow] duration-150 hover:shadow-md hover:shadow-black/30";

type Props = {
  page: number;
  totalPages: number;
  onPageChange: (page: number) => void;
  // Given both, a "Rows per page" select is shown.
  pageSize?: number;
  onPageSizeChange?: (size: number) => void;
};

// Previous / Next under a table, always shown so the page count is visible; with one page both are disabled.
export function Pagination({ page, totalPages, onPageChange, pageSize, onPageSizeChange }: Props) {
  const id = useId();
  const pages = Math.max(totalPages, 1);
  return (
    <nav aria-label="Pagination" className="mt-4 flex flex-wrap items-center justify-between gap-4 border-t pt-4">
      {onPageSizeChange ? (
        <div className="flex items-center gap-2">
          <label htmlFor={id} className="text-sm whitespace-nowrap text-muted-foreground">
            Rows per page
          </label>
          <NativeSelect id={id} className="w-20" value={pageSize} onChange={(e) => onPageSizeChange(Number(e.target.value))}>
            {PAGE_SIZES.map((n) => (
              <option key={n} value={n}>
                {n}
              </option>
            ))}
          </NativeSelect>
        </div>
      ) : (
        <span />
      )}
      <div className="flex items-center gap-3">
        <p role="status" className="text-sm text-muted-foreground tabular-nums">
          Page {page} of {pages}
        </p>
        <div className="flex gap-2">
          <Button variant="outline" className={lift} disabled={page <= 1} onClick={() => onPageChange(page - 1)}>
            <ChevronLeft aria-hidden="true" />
            Previous
          </Button>
          <Button variant="outline" className={lift} disabled={page >= pages} onClick={() => onPageChange(page + 1)}>
            Next
            <ChevronRight aria-hidden="true" />
          </Button>
        </div>
      </div>
    </nav>
  );
}

// One page of rows held in the browser; a page past the end (the rows shrank) shows the last page.
export function pageOf<T>(rows: T[], page: number, size: number) {
  const totalPages = Math.max(1, Math.ceil(rows.length / size));
  const current = Math.min(page, totalPages);
  return { rows: rows.slice((current - 1) * size, current * size), page: current, totalPages };
}
