import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { EmptyState } from "./empty-state";

describe("EmptyState", () => {
  it("explains the empty view and exposes its call to action", () => {
    render(<EmptyState title="No reports found" description="Create one to get started." action={{ href: "/reports/new", label: "Create report" }} />);
    expect(screen.getByRole("status")).toHaveTextContent("No reports found");
    expect(screen.getByRole("link", { name: "Create report" })).toHaveAttribute("href", "/reports/new");
  });
});
