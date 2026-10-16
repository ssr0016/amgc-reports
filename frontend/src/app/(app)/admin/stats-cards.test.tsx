import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/api";
import { StatsCards } from "./stats-cards";

const h = vi.hoisted(() => ({ isAdmin: true, getStats: vi.fn() }));
vi.mock("@/lib/session", () => ({ useSession: () => ({ isAdmin: h.isAdmin }) }));
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
  h.isAdmin = true;
  h.getStats.mockReset();
});

describe("StatsCards", () => {
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

  it("does not fetch, or render anything, for a non-admin", () => {
    h.isAdmin = false;
    const { container } = renderCards();
    expect(h.getStats).not.toHaveBeenCalled();
    expect(container).toBeEmptyDOMElement();
  });
});
