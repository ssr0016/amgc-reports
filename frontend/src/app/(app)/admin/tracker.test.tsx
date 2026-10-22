import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/api";
import type { Tracker as TrackerBody, TrackerRow } from "@/lib/tracker";
import { Tracker } from "./tracker";

const h = vi.hoisted(() => ({ getTracker: vi.fn() }));
// Keep the real labels and counts; replace only the network call.
vi.mock("@/lib/tracker", async (orig) => ({
  ...(await orig<typeof import("@/lib/tracker")>()),
  getTracker: h.getTracker,
}));
// Pin "now" so the default month is known: 22 October 2026 in Manila, so September.
vi.mock("@/lib/reports", async (orig) => {
  const real = await orig<typeof import("@/lib/reports")>();
  return { ...real, defaultPeriod: () => real.defaultPeriod(new Date("2026-10-22T08:00:00Z")) };
});

const row = (user_id: number, name: string, church_name: string, status: TrackerRow["status"], user_status = "active"): TrackerRow => ({
  user_id, name, email: `${name.toLowerCase()}@example.org`, church_name, user_status, status,
});

const ROWS = [
  row(1, "Ana", "Antioch", "approved"),
  row(2, "Bob", "Bethel", "for_review"),
  row(3, "Cora", "Calvary", "not_submitted", "deceased"),
  row(4, "Dan", "", "not_submitted"),
];
const body = (rows: TrackerRow[] = ROWS): TrackerBody => ({ month: "September", year: 2026, rows });

function renderTracker() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const user = userEvent.setup();
  render(
    <QueryClientProvider client={client}>
      <Tracker />
    </QueryClientProvider>,
  );
  return { user };
}

const names = () =>
  screen.getAllByRole("row").slice(1).map((r) => within(r).getAllByRole("cell")[0].textContent);
const count = (label: string) => screen.getByText(label, { selector: "dt" }).nextElementSibling?.textContent;

beforeEach(() => {
  vi.clearAllMocks();
  h.getTracker.mockResolvedValue(body());
});

describe("tracker", () => {
  it("shows loading, then starts on the previous Manila month", async () => {
    renderTracker();
    expect(screen.getByRole("status")).toHaveTextContent("Loading…");
    await screen.findByText("Ana");
    expect(screen.getByLabelText("Month")).toHaveValue("September");
    expect(screen.getByLabelText("Year")).toHaveValue("2026");
    expect(h.getTracker).toHaveBeenLastCalledWith("September", 2026);
  });

  it("shows the counts, the progress and every worker with their status", async () => {
    renderTracker();
    await screen.findByText("Ana");
    expect(count("Expected")).toBe("4");
    expect(count("Submitted")).toBe("2");
    expect(count("Approved")).toBe("1");
    expect(count("For Review")).toBe("1");
    expect(count("Not Submitted")).toBe("2");
    const bar = screen.getByRole("progressbar", { name: "2 of 4 submitted" });
    expect(bar).toHaveAttribute("value", "2");
    expect(bar).toHaveAttribute("max", "4");
    expect(screen.getByText("4 of 4 workers")).toBeInTheDocument();

    const cora = screen.getByText("cora@example.org").closest("tr")!;
    expect(within(cora).getByText("Deceased")).toBeInTheDocument();
    expect(within(cora).getByText("Not Submitted")).toBeInTheDocument();
    // An active account has no status badge; a blank church is a dash.
    const dan = screen.getByText("dan@example.org").closest("tr")!;
    expect(within(dan).queryByTestId("user-status")).not.toBeInTheDocument();
    expect(within(dan).getByText("—")).toBeInTheDocument();
  });

  it("searches name, email and church, without changing the counts", async () => {
    const { user } = renderTracker();
    await screen.findByText("Ana");
    await user.type(screen.getByLabelText("Search"), "bethel");
    expect(names()).toEqual(["Bobbob@example.org"]);
    await user.clear(screen.getByLabelText("Search"));
    await user.type(screen.getByLabelText("Search"), "CORA@");
    expect(names()).toEqual(["CoraDeceasedcora@example.org"]);
    expect(screen.getByText("1 of 4 workers")).toBeInTheDocument();
    expect(count("Expected")).toBe("4");
  });

  it("filters by status", async () => {
    const { user } = renderTracker();
    await screen.findByText("Ana");
    await user.selectOptions(screen.getByLabelText("Status"), "not_submitted");
    expect(names()).toHaveLength(2);
    expect(screen.getByText("2 of 4 workers")).toBeInTheDocument();
    expect(screen.getByRole("progressbar", { name: "2 of 4 submitted" })).toBeInTheDocument();
  });

  it("says when no worker matches", async () => {
    const { user } = renderTracker();
    await screen.findByText("Ana");
    await user.type(screen.getByLabelText("Search"), "nobody");
    expect(screen.getByText("No workers match.")).toBeInTheDocument();
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
  });

  it("says when there is nobody to track, with no progress bar", async () => {
    h.getTracker.mockResolvedValue(body([]));
    renderTracker();
    expect(await screen.findByText("No church workers to track.")).toBeInTheDocument();
    expect(screen.queryByRole("progressbar")).not.toBeInTheDocument();
  });

  it("refetches for a new month", async () => {
    const { user } = renderTracker();
    await screen.findByText("Ana");
    await user.selectOptions(screen.getByLabelText("Month"), "August");
    expect(h.getTracker).toHaveBeenLastCalledWith("August", 2026);
  });

  it("shows an error with Retry", async () => {
    h.getTracker.mockRejectedValueOnce(new Error("Server unavailable"));
    const { user } = renderTracker();
    expect(await screen.findByRole("alert")).toHaveTextContent("Server unavailable");
    await user.click(screen.getByRole("button", { name: "Retry" }));
    expect(await screen.findByText("Ana")).toBeInTheDocument();
  });

  it("tells someone without access they are not allowed", async () => {
    h.getTracker.mockRejectedValue(new ApiError(403, "FORBIDDEN", "Forbidden"));
    renderTracker();
    expect(await screen.findByText("You are not allowed to view the tracker.")).toBeInTheDocument();
  });
});
