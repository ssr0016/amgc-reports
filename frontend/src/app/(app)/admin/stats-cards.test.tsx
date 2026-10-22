import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/api";
import { StatsCards } from "./stats-cards";

const h = vi.hoisted(() => ({ perms: ["tracker:read"] as string[], getStats: vi.fn() }));
vi.mock("@/lib/session", () => ({ useSession: () => ({ can: (p: string) => h.perms.includes(p) }) }));
vi.mock("@/lib/stats", () => ({ getStats: h.getStats }));

const stats = (over = {}) => ({
  month: "September",
  year: 2026,
  reports_total: 3,
  reports_approved: 1,
  reports_pending: 2,
  users_total: 7,
  admins_total: 2,
  ...over,
});

const renderCards = () =>
  render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <StatsCards />
    </QueryClientProvider>,
  );

// Each card is its label, then its number, in reading order.
const card = (label: string) => screen.getByText(label, { selector: "[data-slot=card-description]" }).closest("[data-slot=card]") as HTMLElement;

beforeEach(() => {
  h.perms = ["tracker:read"];
  h.getStats.mockReset();
});

describe("StatsCards", () => {
  it("shows a failed load with Retry, and Retry loads the cards", async () => {
    h.getStats.mockRejectedValueOnce(new Error("Server error.")).mockResolvedValue(stats());
    renderCards();
    expect(await screen.findByRole("alert")).toHaveTextContent("Server error.");
    await userEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(await screen.findByText("Pending", { selector: "[data-slot=card-description]" })).toBeInTheDocument();
    expect(h.getStats).toHaveBeenCalledTimes(2);
  });

  it("announces loading as a status message", () => {
    h.getStats.mockReturnValue(new Promise(() => {}));
    renderCards();
    expect(screen.getByRole("status")).toHaveTextContent("Loading…");
  });

  it("shows the four cards with their numbers and the period", async () => {
    h.getStats.mockResolvedValue(stats());
    renderCards();

    expect(await screen.findByRole("region", { name: "Statistics" })).toBeInTheDocument();
    expect(within(card("Reports")).getByText("3")).toBeInTheDocument();
    expect(within(card("Reports")).getByText("September 2026")).toBeInTheDocument();
    expect(within(card("Approved")).getByText("1")).toBeInTheDocument();
    expect(within(card("Pending")).getByText("2")).toBeInTheDocument();
    expect(within(card("Users")).getByText("7")).toBeInTheDocument();
    expect(within(card("Users")).getByText("2 admins")).toBeInTheDocument();
  });

  it("shows zero as 0, not blank", async () => {
    h.getStats.mockResolvedValue(stats({ reports_total: 0, reports_approved: 0, reports_pending: 0 }));
    renderCards();
    await screen.findByRole("region", { name: "Statistics" });
    for (const label of ["Reports", "Approved", "Pending"]) {
      expect(within(card(label)).getByText("0")).toBeInTheDocument();
    }
  });

  it("says '1 admin' for one", async () => {
    h.getStats.mockResolvedValue(stats({ users_total: 1, admins_total: 1 }));
    renderCards();
    expect(await screen.findByText("1 admin")).toBeInTheDocument();
  });

  it("announces an error with the API's message", async () => {
    h.getStats.mockRejectedValue(new ApiError(500, "INTERNAL_ERROR", "failed to get stats"));
    renderCards();
    expect(await screen.findByRole("alert")).toHaveTextContent("failed to get stats");
    expect(screen.queryByRole("region", { name: "Statistics" })).not.toBeInTheDocument();
  });

  it("does not fetch, or render anything, without tracker:read, even for a plain user who reads every report", () => {
    h.perms = ["reports:read", "reports:read_any", "users:read"];
    const { container } = renderCards();
    expect(h.getStats).not.toHaveBeenCalled();
    expect(container).toBeEmptyDOMElement();
  });
});
