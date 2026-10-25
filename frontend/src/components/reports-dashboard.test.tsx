import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { ReportsTableCard } from "./reports-dashboard";
import type { Report } from "@/lib/reports";

vi.mock("next/font/google", () => ({ Inter: () => ({ className: "inter" }), JetBrains_Mono: () => ({ variable: "mono" }) }));

const rows = [
  { id: 1, worker: "Maria Santos", month: "September", year: 2026, completed: false },
  { id: 2, worker: "Jose Reyes", month: "September", year: 2026, completed: true },
] as Report[];

function table(scope = "page-1", selectable = true) {
  return <ReportsTableCard rows={rows} scope={scope} selectable={selectable} filters={<span>Filters</span>} actions={() => <span>Preview</span>}><span>Pagination</span></ReportsTableCard>;
}

describe("reports table", () => {
  it("selects visible rows, exposes mixed selection, and clears on a page change", async () => {
    const user = userEvent.setup();
    const { rerender } = render(table());
    await user.click(screen.getByRole("checkbox", { name: /Select Maria/ }));
    expect(screen.getByRole("checkbox", { name: "Select all visible reports" })).toBePartiallyChecked();
    expect(within(screen.getByRole("complementary", { name: "Bulk Actions" })).getByRole("status")).toHaveTextContent("1 selected");
    rerender(table("page-2"));
    expect(screen.queryByRole("complementary", { name: "Bulk Actions" })).not.toBeInTheDocument();
    expect(screen.getByRole("checkbox", { name: /Select Maria/ })).not.toBeChecked();
  });

  it("filters the current page and selects only visible rows", async () => {
    const user = userEvent.setup();
    render(table());
    await user.click(screen.getByRole("button", { name: "Approved" }));
    expect(screen.queryByText("Maria Santos")).not.toBeInTheDocument();
    expect(screen.getByText("Jose Reyes")).toBeInTheDocument();
    await user.click(screen.getByRole("checkbox", { name: "Select all visible reports" }));
    expect(screen.getByRole("status")).toHaveTextContent("1 selected");
    await user.click(screen.getByRole("button", { name: "Clear selection" }));
    expect(screen.queryByRole("complementary")).not.toBeInTheDocument();
  });

  it("keeps selection controls hidden without action permissions", () => {
    render(table("page-1", false));
    expect(screen.queryByRole("checkbox")).not.toBeInTheDocument();
    expect(screen.getAllByRole("columnheader").map((cell) => cell.textContent)).toEqual(["Worker", "Month", "Year", "Area", "Church", "Status", "Actions"]);
  });
});
