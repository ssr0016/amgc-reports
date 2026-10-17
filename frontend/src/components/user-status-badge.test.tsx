import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { UserStatusBadge } from "./user-status-badge";

describe("UserStatusBadge", () => {
  // Colour is never the only cue (WCAG 1.4.1): every state prints its label.
  it.each([
    ["active", "Active"],
    ["inactive", "Inactive"],
    ["disciplinary", "Disciplinary"],
    ["deceased", "Deceased"],
  ])("%s reads %s", (status, label) => {
    render(<UserStatusBadge status={status} />);
    expect(screen.getByText(label)).toHaveAttribute("data-status", status);
  });

  it("gives each state its own colour classes", () => {
    const classes = ["active", "inactive", "disciplinary", "deceased"].map((s) => {
      const { unmount } = render(<UserStatusBadge status={s} />);
      const cls = screen.getByTestId("user-status").className;
      unmount();
      return cls;
    });
    expect(new Set(classes).size).toBe(4);
  });

  it("shows a value it does not know as plain text, not a crash", () => {
    render(<UserStatusBadge status="retired" />);
    expect(screen.getByText("retired")).toBeInTheDocument();
  });
});
