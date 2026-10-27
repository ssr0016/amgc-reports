import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { pageOf, Pagination } from "./pagination";

describe("pageOf", () => {
  const rows = Array.from({ length: 23 }, (_, i) => i);

  it("slices one page and counts the pages", () => {
    expect(pageOf(rows, 3, 10)).toEqual({ rows: [20, 21, 22], page: 3, totalPages: 3 });
  });

  it("a page past the end shows the last page; no rows is one empty page", () => {
    expect(pageOf(rows, 9, 10).page).toBe(3);
    expect(pageOf([], 1, 10)).toEqual({ rows: [], page: 1, totalPages: 1 });
  });
});

describe("Pagination", () => {
  it("still shows a single page, with Previous and Next disabled", () => {
    render(<Pagination page={1} totalPages={1} onPageChange={vi.fn()} />);
    expect(screen.getByText("Page 1 of 1")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Previous" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Next" })).toBeDisabled();
  });

  it("pages and changes the page size", () => {
    const onPage = vi.fn();
    const onSize = vi.fn();
    render(<Pagination page={2} totalPages={2} onPageChange={onPage} pageSize={25} onPageSizeChange={onSize} />);
    expect(screen.getByText("Page 2 of 2")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Next" })).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: "Previous" }));
    expect(onPage).toHaveBeenCalledWith(1);
    fireEvent.change(screen.getByLabelText("Rows per page"), { target: { value: "50" } });
    expect(onSize).toHaveBeenCalledWith(50);
  });
});
