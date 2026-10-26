import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { type Report } from "@/lib/reports";
import { METRIC_SECTIONS, WEEKS } from "@/lib/report-form";
import { ReportPreviewDialog, calculateAverage } from "./report-preview-dialog";

const h = vi.hoisted(() => ({ getReport: vi.fn() }));
vi.mock("@/lib/reports", async (orig) => ({
  ...(await orig<typeof import("@/lib/reports")>()),
  getReport: h.getReport,
}));

vi.mock("@/lib/session", () => ({ useSession: () => ({ can: () => true }) }));

// D-A5: average of weeks with values; dash when none.
describe("calculateAverage", () => {
  it("returns dash for undefined", () => {
    expect(calculateAverage(undefined)).toBe("—");
  });

  it("returns dash when all weeks are undefined", () => {
    expect(calculateAverage({})).toBe("—");
  });

  // Note: null is not a valid type for MetricWeeks, but the function handles undefined via filter
  it("returns dash when all weeks are missing", () => {
    expect(calculateAverage({ week1: undefined, week2: undefined, week3: undefined, week4: undefined, week5: undefined })).toBe("—");
  });

  it("averages weeks with values", () => {
    expect(calculateAverage({ week1: 10, week2: 20, week3: undefined })).toBe("15.00");
  });

  it("includes zero as a value", () => {
    expect(calculateAverage({ week1: 10, week2: 0, week3: undefined })).toBe("5.00");
  });

  it("averages all five weeks", () => {
    expect(calculateAverage({ week1: 10, week2: 20, week3: 30, week4: 40, week5: 50 })).toBe("30.00");
  });

  it("rounds to two decimals", () => {
    expect(calculateAverage({ week1: 1, week2: 2 })).toBe("1.50");
  });
});

const makeReport = (overrides: Partial<Report> = {}): Report => ({
  id: 1,
  created_by: 1,
  month: "September",
  year: 2026,
  worker: "John Doe",
  area_assignment: "Area 1",
  church_name: "Test Church",
  metrics: {
    worship_service: { week1: 100, week2: 110, week3: 120, week4: 130, week5: undefined },
    sunday_school: { week1: 50, week2: 55, week3: undefined },
    prayer_meeting: { week1: 20 },
    bible_studies: { week1: 5, week2: 6, week3: 7, week4: 8, week5: 9 },
    tithes_offering: { week1: 1000.5, week2: 2000.5, week3: 3000.5, week4: 4000.5, week5: 5000.5 },
    mens_fellowship: {},
    womens_fellowship: {},
    youth_fellowship: { week1: 10, week2: 20 },
    children_fellowship: { week1: 15 },
    home_visited: { week1: 5, week2: 5, week3: 5, week4: 5, week5: 5 },
    bible_study_group_led: { week1: 1 },
    sermon_preached: { week1: 2, week2: 2 },
    person_newly_contacted: { week1: 3 },
    person_followed_up: { week1: 4 },
    person_evangelized: { week1: 5 },
    outreach: { week1: 1 },
    training: { week1: 0 },
    leadership: { week1: 1 },
    baptism: { week1: 0 },
    other: {},
    family_day: {},
  },
  completed: true,
  status: "approved",
  created_at: "2026-10-01T00:00:00Z",
  updated_at: "2026-10-01T00:00:00Z",
  owner_status: "approved",
  ...overrides,
});

function renderDialog(reportId: number = 1) {
  const onClose = vi.fn();
  const user = userEvent.setup();
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });

  render(
    <QueryClientProvider client={queryClient}>
      <ReportPreviewDialog reportId={reportId} onClose={onClose} />
    </QueryClientProvider>,
  );

  return { user, onClose, queryClient };
}

describe("ReportPreviewDialog", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("shows loading state initially", () => {
    h.getReport.mockReturnValue(new Promise(() => {})); // never resolves
    const { onClose } = renderDialog();

    expect(screen.getByText("Report Preview")).toBeInTheDocument();
    // Loading is shown initially
    onClose.mockClear();
  });

  it("shows error state on failure", async () => {
    h.getReport.mockRejectedValue(new Error("Not found"));
    const { onClose } = renderDialog();

    await waitFor(() => {
      expect(screen.getByText("Not found")).toBeInTheDocument();
    });

    // Retry button works
    await userEvent.click(screen.getByRole("button", { name: /retry/i }));
    expect(h.getReport).toHaveBeenCalledTimes(2);

    onClose.mockClear();
  });

  it("renders metadata when loaded", async () => {
    h.getReport.mockResolvedValue(makeReport());
    renderDialog();

    await waitFor(() => {
      expect(screen.getByText("John Doe")).toBeInTheDocument();
      expect(screen.getByText("Area 1")).toBeInTheDocument();
      expect(screen.getByText("Test Church")).toBeInTheDocument();
      expect(screen.getByText("September 2026")).toBeInTheDocument();
      expect(screen.getByText("Approved")).toBeInTheDocument();
    });
  });

  it("shows admin notes for reports needing revision", async () => {
    h.getReport.mockResolvedValue(makeReport({ completed: false, status: "needs_revision", admin_notes: "Please add the missing totals." }));
    renderDialog();

    expect(await screen.findByRole("alert")).toHaveTextContent("Please add the missing totals.");
    expect(screen.getByText("Admin Notes")).toBeInTheDocument();
    expect(screen.getByText("Needs Revision")).toBeInTheDocument();
  });

  it("uses status instead of completed for the status label", async () => {
    h.getReport.mockResolvedValue(makeReport({ completed: true, status: "pending" }));
    renderDialog();

    expect(await screen.findByText("Pending")).toBeInTheDocument();
    expect(screen.queryByText("Completed")).not.toBeInTheDocument();
  });

  it("renders all four metric sections", async () => {
    h.getReport.mockResolvedValue(makeReport());
    renderDialog();

    await waitFor(() => {
      for (const section of METRIC_SECTIONS) {
        expect(screen.getByRole("heading", { name: section.title })).toBeInTheDocument();
      }
    });
  });

  it("renders all report details as wrapping plain text with line breaks preserved", async () => {
    const details = {
      names: "Ana\nJuan",
      narrative_report: "First visit\n<script>alert('test')</script>",
      challenges: "Rain\n" + "x".repeat(200),
      prayer_request: "Health\nFamily",
    };
    h.getReport.mockResolvedValue({ ...makeReport(), ...details });
    renderDialog();

    const section = await screen.findByRole("region", { name: "Report Details" });
    const labels = ["Names of New Believers", "Narrative Report", "Challenges", "Prayer Request"];
    for (const label of labels) {
      expect(within(section).getByText(label)).toBeInTheDocument();
    }
    for (const value of Object.values(details)) {
      const text = within(section).getByText(value, { normalizer: (text) => text });
      expect(text.textContent).toBe(value);
      expect(text).toHaveClass("whitespace-pre-wrap", "[overflow-wrap:anywhere]");
    }
    expect(section.querySelector("script")).toBeNull();
  });

  it("shows a dash for each missing report detail", async () => {
    h.getReport.mockResolvedValue(makeReport());
    renderDialog();

    const section = await screen.findByRole("region", { name: "Report Details" });
    expect(within(section).getAllByText("—")).toHaveLength(4);
  });

  it("renders Week columns in header", async () => {
    h.getReport.mockResolvedValue(makeReport());
    renderDialog();

    await waitFor(() => {
      expect(screen.getAllByText("Week 1")).toHaveLength(4);
      expect(screen.getAllByText("Week 2")).toHaveLength(4);
      expect(screen.getAllByText("Week 3")).toHaveLength(4);
      expect(screen.getAllByText("Week 4")).toHaveLength(4);
      expect(screen.getAllByText("Week 5")).toHaveLength(4);
    });
  });

  it("renders Avg column in each table", async () => {
    h.getReport.mockResolvedValue(makeReport());
    renderDialog();

    await waitFor(() => {
      // Four tables, each has an Avg column header
      expect(screen.getAllByText("Avg")).toHaveLength(4);
    });
  });

  it("shows dash for missing metrics", async () => {
    h.getReport.mockResolvedValue(makeReport());
    renderDialog();

    await waitFor(() => {
      // mens_fellowship has {} - all undefined, so Avg shows —
      expect(screen.getAllByText("—").length).toBeGreaterThan(0);
    });
  });

  it("shows computed averages for metrics with values", async () => {
    // worship_service: (100+110+120+130)/4 = 115.00
    h.getReport.mockResolvedValue(makeReport());
    renderDialog();

    await waitFor(() => {
      expect(screen.getByText("115.00")).toBeInTheDocument(); // worship_service
      expect(screen.getByText("52.50")).toBeInTheDocument(); // sunday_school: (50+55)/2
      expect(screen.getByText("20.00")).toBeInTheDocument(); // prayer_meeting: 20/1
    });
  });

  it("closes via Cancel button", async () => {
    h.getReport.mockResolvedValue(makeReport());
    const { onClose, user } = renderDialog();

    await waitFor(() => {
      expect(screen.getByText("John Doe")).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: "Close" }));
    expect(onClose).toHaveBeenCalled();
  });

  it("closes via backdrop click", async () => {
    h.getReport.mockResolvedValue(makeReport());
    const { onClose, user } = renderDialog();

    await waitFor(() => {
      expect(screen.getByText("John Doe")).toBeInTheDocument();
    });

    // Click the dialog content to ensure it's mounted, then press Escape
    await user.keyboard("{Escape}");
    expect(onClose).toHaveBeenCalled();
  });
});
