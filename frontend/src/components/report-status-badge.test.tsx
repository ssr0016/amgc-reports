import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ReportStatusBadge } from "./report-status-badge";

describe("ReportStatusBadge", () => {
  it.each([
    ["approved", "Approved", "bg-green-100", "text-green-900"],
    ["pending", "Pending", "bg-amber-100", "text-amber-900"],
    ["needs_revision", "Needs Revision", "bg-red-100", "text-red-900"],
  ])("keeps %s readable in light mode", (status, label, background, foreground) => {
    render(<ReportStatusBadge status={status} />);
    const badge = screen.getByText(label);
    expect(badge).toHaveClass(background, foreground);
  });
});
