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

const row = (user_id: number, name: string, church_name: string, submission_status: TrackerRow["submission_status"], review_status?: TrackerRow["review_status"], user_status = "active"): TrackerRow => ({
  user_id, name, email: `${name.toLowerCase()}@example.org`, church_name, user_status, submission_status, review_status,
});

const ROWS = [
  row(1, "Ana", "Antioch", "submitted", "approved"),
  row(2, "Bob", "Bethel", "submitted", "pending"),
  row(5, "Eva", "Ephesus", "submitted", "needs_revision"),
  row(3, "Cora", "Calvary", "no_report", undefined, "deceased"),
  row(4, "Dan", "", "no_report"),
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
    expect(count("Expected")).toBe("5");
    expect(count("Submitted")).toBe("3");
    expect(count("No Report")).toBe("2");
    expect(count("Approved")).toBe("1");
    expect(count("Pending")).toBe("1");
    expect(count("Needs Revision")).toBe("1");
    const bar = screen.getByRole("progressbar", { name: "3 of 5 submitted" });
    expect(bar).toHaveAttribute("value", "3");
    expect(bar).toHaveAttribute("max", "5");
    expect(screen.getByText("5 of 5 workers")).toBeInTheDocument();

    const cora = screen.getByText("cora@example.org").closest("tr")!;
    expect(within(cora).getByText("Deceased")).toBeInTheDocument();
    expect(within(cora).getByText("No Report")).toBeInTheDocument();
    // An active account has no status badge; a blank church is a dash.
    const dan = screen.getByText("dan@example.org").closest("tr")!;
    expect(within(dan).queryByTestId("user-status")).not.toBeInTheDocument();
    expect(within(dan).getAllByText("—")).toHaveLength(2);
  });

  it("searches name, email and church, without changing the counts", async () => {
    const { user } = renderTracker();
    await screen.findByText("Ana");
    await user.type(screen.getByLabelText("Search"), "bethel");
    expect(names()).toEqual(["Bobbob@example.org"]);
    await user.clear(screen.getByLabelText("Search"));
    await user.type(screen.getByLabelText("Search"), "CORA@");
    expect(names()).toEqual(["CoraDeceasedcora@example.org"]);
    expect(screen.getByText("1 of 5 workers")).toBeInTheDocument();
    expect(count("Expected")).toBe("5");
  });

  it("filters by submission status", async () => {
    const { user } = renderTracker();
    await screen.findByText("Ana");
    await user.selectOptions(screen.getByLabelText("Submission status"), "no_report");
    expect(names()).toHaveLength(2);
    expect(screen.getByText("2 of 5 workers")).toBeInTheDocument();
    expect(screen.getByRole("progressbar", { name: "3 of 5 submitted" })).toBeInTheDocument();
  });

  it("filters review status separately from submission status", async () => {
    const { user } = renderTracker();
    await screen.findByText("Ana");
    await user.selectOptions(screen.getByLabelText("Review status"), "needs_revision");
    expect(names()).toEqual(["Evaeva@example.org"]);
    await user.selectOptions(screen.getByLabelText("Submission status"), "no_report");
    expect(screen.getByText("No workers match.")).toBeInTheDocument();
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
