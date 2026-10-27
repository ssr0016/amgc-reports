import { render, screen, waitFor } from "@testing-library/react";
import Link from "next/link";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { GlobalActivityIndicator } from "./global-activity-indicator";

const h = vi.hoisted(() => ({ fetching: 0 }));
vi.mock("@tanstack/react-query", () => ({ useIsFetching: () => h.fetching }));
vi.mock("next/navigation", () => ({ usePathname: () => "/reports" }));

describe("GlobalActivityIndicator", () => {
  beforeEach(() => {
    h.fetching = 0;
  });

  it("shows a subtle background-fetch indicator only while queries fetch", () => {
    h.fetching = 1;
    render(<GlobalActivityIndicator />);
    expect(screen.getByRole("status")).toHaveTextContent("Updating");
    expect(screen.getByTestId("background-fetch-indicator")).toBeVisible();
  });

  it("starts route progress for an internal link and completes on pathname change", async () => {
    render(<><GlobalActivityIndicator /><Link href="/reports/new">New report</Link></>);
    expect(screen.queryByTestId("route-progress")).not.toBeInTheDocument();
    screen.getByRole("link", { name: "New report" }).dispatchEvent(new MouseEvent("click", { bubbles: true, button: 0 }));
    await waitFor(() => expect(screen.getByTestId("route-progress")).toBeVisible());
  });
});
