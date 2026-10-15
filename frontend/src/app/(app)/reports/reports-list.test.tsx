import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/api";
import type { Page, Report } from "@/lib/reports";
import { Reports } from "./reports-list";

const h = vi.hoisted(() => ({
  push: vi.fn(),
  replace: vi.fn(),
  search: "",
  me: { user_id: 1, perms: [] as string[] },
  listReports: vi.fn(),
  deleteReport: vi.fn(),
  toggleComplete: vi.fn(),
  toast: { success: vi.fn(), error: vi.fn() },
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: h.push, replace: h.replace }),
  useSearchParams: () => new URLSearchParams(h.search),
}));
vi.mock("sonner", () => ({ toast: h.toast }));
vi.mock("@/lib/session", () => ({
  useSession: () => ({ user: { user_id: h.me.user_id }, can: (p: string) => h.me.perms.includes(p) }),
}));
// Keep the real constants and defaultPeriod; replace only the calls that hit the network.
vi.mock("@/lib/reports", async (orig) => ({
  ...(await orig<typeof import("@/lib/reports")>()),
  listReports: h.listReports,
  deleteReport: h.deleteReport,
  toggleComplete: h.toggleComplete,
}));

const report = (id: number, by: number, extra: Partial<Report> = {}): Report =>
  ({
    id, created_by: by, month: "April", year: 2026, metrics: {}, completed: false,
    created_at: "", updated_at: "", worker: `Worker ${id}`, ...extra,
  }) as Report;

const page = (data: Report[], extra: Partial<Page<Report>["meta"]> = {}): Page<Report> =>
  ({ data, meta: { page: 1, limit: 20, total: data.length, total_pages: 1, ...extra } }) as Page<Report>;

function renderList(...perms: string[]) {
  h.me.perms = perms;
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const user = userEvent.setup();
  render(
    <QueryClientProvider client={client}>
      <Reports />
    </QueryClientProvider>,
  );
  return { user, client };
}

const row = (text: string) => screen.getByText(text).closest("tr")!;

beforeEach(() => {
  vi.clearAllMocks();
  h.search = "";
  h.me.user_id = 1;
  h.listReports.mockResolvedValue(page([report(1, 1), report(2, 2)]));
});

describe("reports list: access", () => {
  it("without reports:read it says so and never asks the API", () => {
    renderList();
    expect(screen.getByText("You are not allowed to view reports.")).toBeInTheDocument();
    expect(h.listReports).not.toHaveBeenCalled();
  });

  it("a 403 from the API reads the same", async () => {
    h.listReports.mockRejectedValue(new ApiError(403, "FORBIDDEN", "no"));
    renderList("reports:read");
    expect(await screen.findByText("You are not allowed to view reports.")).toBeInTheDocument();
  });

  it("shows other API errors, and an empty state", async () => {
    h.listReports.mockRejectedValue(new ApiError(500, "ERROR", "Something broke."));
    renderList("reports:read");
    expect(await screen.findByRole("alert")).toHaveTextContent("Something broke.");
  });

  it("says when nothing matches", async () => {
    h.listReports.mockResolvedValue(page([]));
    renderList("reports:read");
    expect(await screen.findByText("No reports match these filters.")).toBeInTheDocument();
  });
});

describe("reports list: gated buttons", () => {
  it("New report needs reports:write", async () => {
    renderList("reports:read");
    await screen.findByText("Worker 1");
    expect(screen.queryByRole("link", { name: "New report" })).not.toBeInTheDocument();
  });

  it("a plain user can edit and delete only their own report", async () => {
    renderList("reports:read", "reports:read_any", "reports:write", "reports:delete");
    await screen.findByText("Worker 1");
    expect(within(row("Worker 1")).getByRole("link", { name: "Edit" })).toBeInTheDocument();
    expect(within(row("Worker 1")).getByRole("button", { name: "Delete" })).toBeInTheDocument();
    expect(within(row("Worker 2")).getByRole("link", { name: "View" })).toBeInTheDocument();
    expect(within(row("Worker 2")).queryByRole("button", { name: "Delete" })).not.toBeInTheDocument();
  });

  it("_any permissions reach other people's reports", async () => {
    renderList("reports:read", "reports:write", "reports:write_any", "reports:delete", "reports:delete_any");
    await screen.findByText("Worker 2");
    expect(within(row("Worker 2")).getByRole("link", { name: "Edit" })).toBeInTheDocument();
    expect(within(row("Worker 2")).getByRole("button", { name: "Delete" })).toBeInTheDocument();
  });

  it("without write or delete, even the owner only views", async () => {
    renderList("reports:read");
    await screen.findByText("Worker 1");
    expect(within(row("Worker 1")).getByRole("link", { name: "View" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Delete" })).not.toBeInTheDocument();
  });

  it("Approve shows only with reports:approve", async () => {
    renderList("reports:read");
    await screen.findByText("Worker 1");
    expect(screen.queryByRole("button", { name: /approve/i })).not.toBeInTheDocument();
  });
});

describe("reports list: approve", () => {
  it("flips the row at once and confirms with the server's answer", async () => {
    let resolve!: (v: unknown) => void;
    h.toggleComplete.mockReturnValue(new Promise((r) => (resolve = r)));
    const { user } = renderList("reports:read", "reports:approve");
    await screen.findByText("Worker 1");

    await user.click(within(row("Worker 1")).getByRole("button", { name: "Approve" }));
    // optimistic: Completed and Unapprove while the request is in flight; the button is held
    expect(within(row("Worker 1")).getByText("Completed")).toBeInTheDocument();
    expect(within(row("Worker 1")).getByRole("button", { name: "Unapprove" })).toBeDisabled();

    resolve({ id: 1, completed: true });
    await waitFor(() => expect(h.toast.success).toHaveBeenCalledWith("Approved April 2026"));
    expect(within(row("Worker 1")).getByRole("button", { name: "Unapprove" })).toBeEnabled();
  });

  it("rolls back and reports a failure", async () => {
    h.toggleComplete.mockRejectedValue(new ApiError(429, "RATE_LIMIT", "Slow down.", undefined, 9));
    const { user } = renderList("reports:read", "reports:approve");
    await screen.findByText("Worker 1");
    await user.click(within(row("Worker 1")).getByRole("button", { name: "Approve" }));
    await waitFor(() => expect(h.toast.error).toHaveBeenCalledWith("Slow down. Try again in 9s."));
    expect(within(row("Worker 1")).getByText("Pending")).toBeInTheDocument();
  });

  it("a 404 says the report is gone and reloads the list", async () => {
    h.toggleComplete.mockRejectedValue(new ApiError(404, "NOT_FOUND", "gone"));
    const { user } = renderList("reports:read", "reports:approve");
    await screen.findByText("Worker 1");
    await user.click(within(row("Worker 1")).getByRole("button", { name: "Approve" }));
    await waitFor(() => expect(h.toast.error).toHaveBeenCalledWith("April 2026 no longer exists."));
    await waitFor(() => expect(h.listReports).toHaveBeenCalledTimes(2));
  });

  it("an approved report offers Unapprove", async () => {
    h.listReports.mockResolvedValue(page([report(1, 1, { completed: true })]));
    renderList("reports:read", "reports:approve");
    expect(await screen.findByRole("button", { name: "Unapprove" })).toBeInTheDocument();
  });
});

describe("reports list: delete", () => {
  const mine = ["reports:read", "reports:write", "reports:delete"];

  it("asks first, then deletes and reloads", async () => {
    h.deleteReport.mockResolvedValue(undefined);
    const { user } = renderList(...mine);
    await screen.findByText("Worker 1");
    await user.click(within(row("Worker 1")).getByRole("button", { name: "Delete" }));
    expect(await screen.findByText("Delete April 2026?")).toBeInTheDocument();
    expect(h.deleteReport).not.toHaveBeenCalled();

    await user.click(within(screen.getByRole("alertdialog")).getByRole("button", { name: "Delete" }));
    await waitFor(() => expect(h.deleteReport).toHaveBeenCalledWith(1));
    await waitFor(() => expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument());
    await waitFor(() => expect(h.listReports).toHaveBeenCalledTimes(2));
  });

  it("Cancel closes the dialog without deleting", async () => {
    const { user } = renderList(...mine);
    await screen.findByText("Worker 1");
    await user.click(within(row("Worker 1")).getByRole("button", { name: "Delete" }));
    await user.click(await screen.findByRole("button", { name: "Cancel" }));
    await waitFor(() => expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument());
    expect(h.deleteReport).not.toHaveBeenCalled();
  });

  it("a failed delete keeps the dialog open with the reason", async () => {
    h.deleteReport.mockRejectedValue(new ApiError(500, "ERROR", "Server error."));
    const { user } = renderList(...mine);
    await screen.findByText("Worker 1");
    await user.click(within(row("Worker 1")).getByRole("button", { name: "Delete" }));
    await user.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: "Delete" }));
    expect(await within(screen.getByRole("alertdialog")).findByRole("alert")).toHaveTextContent("Server error.");
  });

  it("a 404 counts as deleted: the dialog closes", async () => {
    h.deleteReport.mockRejectedValue(new ApiError(404, "NOT_FOUND", "gone"));
    const { user } = renderList(...mine);
    await screen.findByText("Worker 1");
    await user.click(within(row("Worker 1")).getByRole("button", { name: "Delete" }));
    await user.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: "Delete" }));
    await waitFor(() => expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument());
  });

  it("deleting the last row of a later page steps back one page", async () => {
    h.search = "page=3";
    h.listReports.mockResolvedValue(page([report(1, 1)], { page: 3, total_pages: 3, total: 41 }));
    h.deleteReport.mockResolvedValue(undefined);
    const { user } = renderList(...mine);
    await screen.findByText("Worker 1");
    await user.click(within(row("Worker 1")).getByRole("button", { name: "Delete" }));
    await user.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: "Delete" }));
    await waitFor(() => expect(h.push).toHaveBeenCalledWith(expect.stringContaining("page=2")));
  });
});

describe("reports list: filters and pagination", () => {
  it("starts on the previous month in Asia/Manila, this year", async () => {
    renderList("reports:read");
    await screen.findByText("Worker 1");
    const { defaultPeriod } = await import("@/lib/reports");
    const def = defaultPeriod();
    expect(h.listReports).toHaveBeenCalledWith(expect.objectContaining({ month: def.month, year: def.year }));
    expect(screen.getByLabelText("Month")).toHaveValue(def.month);
  });

  it("month= in the URL means any month; page, text filters pass through", async () => {
    h.search = "month=&year=2026&worker=Ana&page=2";
    renderList("reports:read");
    await screen.findByText("Worker 1");
    expect(h.listReports).toHaveBeenCalledWith({
      month: undefined, year: 2026, worker: "Ana", area: undefined, church: undefined, page: 2,
    });
    expect(screen.getByLabelText("Worker")).toHaveValue("Ana");
  });

  it("an out-of-range year falls back to the default", async () => {
    h.search = "year=1999";
    renderList("reports:read");
    await screen.findByText("Worker 1");
    const { defaultPeriod } = await import("@/lib/reports");
    expect(h.listReports).toHaveBeenCalledWith(expect.objectContaining({ year: defaultPeriod().year }));
  });

  it("choosing a month pushes it to the URL", async () => {
    const { user } = renderList("reports:read");
    await screen.findByText("Worker 1");
    await user.selectOptions(screen.getByLabelText("Month"), "March");
    expect(h.push).toHaveBeenCalledWith(expect.stringMatching(/^\/reports\?month=March&year=\d{4}$/));
  });

  it("typing waits for a pause before it changes the URL", async () => {
    renderList("reports:read");
    await screen.findByText("Worker 1");
    fireEvent.change(screen.getByLabelText("Worker"), { target: { value: "An" } });
    fireEvent.change(screen.getByLabelText("Worker"), { target: { value: "Ana" } });
    expect(h.replace).not.toHaveBeenCalled();
    await waitFor(() => expect(h.replace).toHaveBeenCalledTimes(1));
    expect(h.replace).toHaveBeenCalledWith(expect.stringContaining("worker=Ana"));
  });

  it("Clear appears once there is text and empties the filters", async () => {
    h.search = "worker=Ana";
    const { user } = renderList("reports:read");
    await screen.findByText("Worker 1");
    await user.click(screen.getByRole("button", { name: "Clear" }));
    expect(screen.getByLabelText("Worker")).toHaveValue("");
    expect(h.push).toHaveBeenCalledWith(expect.not.stringContaining("worker="));
  });

  it("no Clear button without text", async () => {
    renderList("reports:read");
    await screen.findByText("Worker 1");
    expect(screen.queryByRole("button", { name: "Clear" })).not.toBeInTheDocument();
  });

  it("paginates: Previous is off on page 1, Next goes to page 2", async () => {
    h.listReports.mockResolvedValue(page([report(1, 1)], { total_pages: 3, total: 41 }));
    const { user } = renderList("reports:read");
    expect(await screen.findByText("Page 1 of 3 (41 reports)")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Previous" })).toBeDisabled();
    await user.click(screen.getByRole("button", { name: "Next" }));
    expect(h.push).toHaveBeenCalledWith(expect.stringContaining("page=2"));
  });

  it("Next is off on the last page", async () => {
    h.search = "page=3";
    h.listReports.mockResolvedValue(page([report(1, 1)], { page: 3, total_pages: 3, total: 41 }));
    renderList("reports:read");
    await screen.findByText("Page 3 of 3 (41 reports)");
    expect(screen.getByRole("button", { name: "Next" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Previous" })).toBeEnabled();
  });
});
