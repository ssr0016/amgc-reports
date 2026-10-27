import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/api";
import { emptyForm, type FormState } from "@/lib/report-form";
import { ReportForm } from "./report-form";

const period = { month: "April", year: 2026 };
const initial = (): FormState => ({ ...emptyForm({ worker: "Ana" }), month: "April", year: "2026" });

function setup(props: Partial<React.ComponentProps<typeof ReportForm>> = {}, fakeTimers = false) {
  const onSubmit = vi.fn().mockResolvedValue(undefined);
  const onCancel = vi.fn();
  const user = userEvent.setup(fakeTimers ? { advanceTimers: vi.advanceTimersByTime } : undefined);
  render(
    <ReportForm initial={initial()} submitLabel="Save report" lockedPeriod={period} onSubmit={onSubmit} onCancel={onCancel} {...props} />,
  );
  return { user, onSubmit, onCancel };
}

const metric = (label: string, week: number) => screen.getByLabelText(`${label}, week ${week}`);

beforeEach(() => localStorage.clear());
afterEach(() => {
  vi.useRealTimers();
  vi.restoreAllMocks();
});

describe("ReportForm", () => {
	it("renders profile metadata as read-only text", () => {
		setup({ initial: { ...initial(), area_assignment: "North", church_name: "AMGC Church" } });
		expect(screen.getByText("Worker Profile")).toBeInTheDocument();
		expect(screen.getByText("Ana")).toBeInTheDocument();
		expect(screen.getByText("North")).toBeInTheDocument();
		expect(screen.getByText("AMGC Church")).toBeInTheDocument();
		expect(screen.queryByLabelText("Worker Name")).not.toBeInTheDocument();
		expect(screen.queryByLabelText("Area Assignment")).not.toBeInTheDocument();
		expect(screen.queryByLabelText("Church Name")).not.toBeInTheDocument();
	});

	it("shows only the submit action in the sticky footer", () => {
		setup({ draft: true, onAutoSave: vi.fn().mockResolvedValue(undefined) });
		expect(screen.queryByRole("button", { name: "Save Draft" })).not.toBeInTheDocument();
		expect(screen.getByRole("button", { name: "Submit Report" })).toBeInTheDocument();
		expect(screen.getByRole("button", { name: "Cancel" })).toBeInTheDocument();
	});

	it("places an end action on the far side of the footer on larger screens", () => {
		setup({ endAction: <button type="button">Delete Report</button> });

		expect(screen.getByRole("button", { name: "Delete Report" }).parentElement).toHaveClass("sm:ml-auto");
	});

	it("labels the single action Resubmit Report for revisions", () => {
		setup({ revision: true });
		expect(screen.getByRole("button", { name: "Resubmit Report" })).toBeInTheDocument();
		expect(screen.queryByRole("button", { name: "Save Changes" })).not.toBeInTheDocument();
	});

	it("recovers local changes and discards them on request", async () => {
		const recovered = { ...initial(), challenges: "Keep this locally" };
    localStorage.setItem("report:April:2026", JSON.stringify({ context: "report:April:2026", form: recovered, user_notes: "Explain the revision" }));
    const { user } = setup({ storageKey: "report:April:2026", revision: true });
		expect(await screen.findByText("Recovered unsaved changes.")).toBeInTheDocument();
		expect(screen.getByLabelText("Challenges")).toHaveValue("Keep this locally");
    expect(screen.getByLabelText("Your Notes / Explanation for Revision")).toHaveValue("Explain the revision");
		await user.click(screen.getByRole("button", { name: "Discard" }));
		expect(screen.getByLabelText("Challenges")).toHaveValue("");
    expect(screen.getByLabelText("Your Notes / Explanation for Revision")).toHaveValue("");
		expect(localStorage.getItem("report:April:2026")).toBeNull();
	});

	it("ignores local changes saved for a different form context", async () => {
		localStorage.setItem("report:April:2026", JSON.stringify({ context: "report:12", form: { ...initial(), challenges: "Deleted report" }, userNotes: "" }));
		setup({ storageKey: "report:April:2026" });
		expect(screen.queryByText("Recovered unsaved changes.")).not.toBeInTheDocument();
		expect(screen.getByLabelText("Challenges")).toHaveValue("");
		expect(localStorage.getItem("report:April:2026")).toBeNull();
	});

	it("recovers unsaved changes when the context matches", async () => {
		localStorage.setItem("report:April:2026", JSON.stringify({ context: "report:April:2026", form: { ...initial(), challenges: "Current period" }, userNotes: "" }));
		setup({ storageKey: "report:April:2026" });
		expect(await screen.findByText("Recovered unsaved changes.")).toBeInTheDocument();
		expect(screen.getByLabelText("Challenges")).toHaveValue("Current period");
	});

	it("debounces server autosave until three seconds after the last change", async () => {
		vi.useFakeTimers();
		const onAutoSave = vi.fn().mockResolvedValue(undefined);
		setup({ storageKey: "report:April:2026", onAutoSave });
		fireEvent.change(screen.getByLabelText("Challenges"), { target: { value: "x" } });
		vi.advanceTimersByTime(2999);
		expect(onAutoSave).not.toHaveBeenCalled();
		await vi.advanceTimersByTimeAsync(1);
		expect(onAutoSave).toHaveBeenCalledOnce();
		expect(onAutoSave.mock.calls[0][0]).toMatchObject({ challenges: "x" });
	});

  it("preserves revision notes in localStorage and server autosave when another field changes", async () => {
    vi.useFakeTimers();
    const onAutoSave = vi.fn().mockResolvedValue(undefined);
    setup({ storageKey: "report:April:2026", onAutoSave, revision: true });
    fireEvent.change(screen.getByLabelText("Your Notes / Explanation for Revision"), { target: { value: "Updated weekly totals" } });
    fireEvent.change(screen.getByLabelText("Challenges"), { target: { value: "x" } });

    expect(JSON.parse(localStorage.getItem("report:April:2026")!).user_notes).toBe("Updated weekly totals");
    await vi.advanceTimersByTimeAsync(3000);

    expect(onAutoSave.mock.calls[0][0]).toMatchObject({ challenges: "x", user_notes: "Updated weekly totals" });
  });

	it("shows the time of the last successful autosave", async () => {
		vi.useFakeTimers();
		vi.setSystemTime(new Date("2026-10-27T10:30:00Z"));
		setup({ storageKey: "report:April:2026", onAutoSave: vi.fn().mockResolvedValue(undefined) });
		fireEvent.change(screen.getByLabelText("Challenges"), { target: { value: "x" } });
		await act(() => vi.advanceTimersByTimeAsync(3000));
		expect(screen.getByText(/Auto-saved at/)).toBeInTheDocument();
	});

	it("explains how to leave inactive weeks blank", () => {
		setup();
		expect(screen.getByText("Fill in only the weeks when an activity happened. Leave a week blank when there was no activity; enter 0 when the activity happened but the count was zero.")).toBeInTheDocument();
	});

  it("describes each long-text field with its help line", () => {
    setup();
    expect(screen.getByLabelText("Narrative Report")).toHaveAccessibleDescription(
      "Describe what happened this month in your ministry. Include key events, milestones, or testimonies.",
    );
    expect(screen.getByLabelText("Names of New Believers")).toHaveAccessibleDescription(
      "e.g., Juan dela Cruz, Maria Santos (Separate names with commas).",
    );
    expect(screen.getByLabelText("Prayer Request")).toHaveAccessibleDescription(
      "List your prayer requests here so the team can pray for you.",
    );
  });

  it("submits the typed values, sending only the weeks that have a value", async () => {
    const { user, onSubmit } = setup();
    await user.type(metric("Outreach", 1), "0");
    await user.type(metric("Outreach", 3), "4");
    await user.type(screen.getByLabelText("Narrative Report"), "A good month");
    await user.click(screen.getByRole("button", { name: "Save report" }));

    expect(onSubmit).toHaveBeenCalledOnce();
    const input = onSubmit.mock.calls[0][0];
    expect(input).toMatchObject({ month: "April", year: 2026, worker: "Ana", narrative_report: "A good month" });
    expect(input.metrics).toEqual({ outreach: { week1: 0, week3: 4 } });
  });

  it("checks a metric on blur and clears the message once it is fixed", async () => {
    const { user } = setup();
    await user.type(metric("Outreach", 2), "1.5");
    await user.tab();
    expect(await screen.findByText("Must be a whole number")).toBeInTheDocument();
    expect(metric("Outreach", 2)).toHaveAttribute("aria-invalid", "true");

    await user.clear(metric("Outreach", 2));
    await user.type(metric("Outreach", 2), "2");
    expect(screen.queryByText("Must be a whole number")).not.toBeInTheDocument();
  });

  it("does not submit an invalid form and says why", async () => {
    const { user, onSubmit } = setup();
    await user.type(metric("Tithes & Offering", 1), "1.234");
    await user.click(screen.getByRole("button", { name: "Save report" }));
    expect(onSubmit).not.toHaveBeenCalled();
    expect(screen.getAllByRole("alert").map((a) => a.textContent).join(" ")).toMatch(/Fix the highlighted fields/);
  });

  it("locks the period: month disabled, year read-only", () => {
    setup();
    expect(screen.getByLabelText("Month")).toBeDisabled();
    expect(screen.getByLabelText("Year")).toHaveAttribute("readonly");
    expect(screen.getByText("Fixed for this report.")).toBeInTheDocument();
  });

  it("lets a new period be chosen when nothing is locked", async () => {
    const { user, onSubmit } = setup({ lockedPeriod: undefined });
    await user.selectOptions(screen.getByLabelText("Month"), "June");
    await user.click(screen.getByRole("button", { name: "Save report" }));
    expect(onSubmit.mock.calls[0][0]).toMatchObject({ month: "June" });
  });

  it("maps a 409 onto the month field and keeps what was typed", async () => {
    const onSubmit = vi.fn().mockRejectedValue(new ApiError(409, "CONFLICT", "exists"));
    const { user } = setup({ onSubmit });
    await user.type(screen.getByLabelText("Narrative Report"), "keep me");
    await user.click(screen.getByRole("button", { name: "Save report" }));
    expect(await screen.findByText("You already have a report for this month and year.")).toBeInTheDocument();
    expect(screen.getByLabelText("Narrative Report")).toHaveValue("keep me");
    expect(screen.getByRole("button", { name: "Save report" })).toBeEnabled();
  });

  it("maps 422 details onto their fields", async () => {
    const err = new ApiError(422, "VALIDATION_ERROR", "bad", { challenges: "Too long", "metrics[outreach]": "Bad metric" });
    const { user } = setup({ onSubmit: vi.fn().mockRejectedValue(err) });
    await user.click(screen.getByRole("button", { name: "Save report" }));
    expect(await screen.findByText("Too long")).toBeInTheDocument();
    expect(screen.getByText("Bad metric")).toBeInTheDocument();
  });

  it("a field error and the help line both describe the field", async () => {
    const err = new ApiError(422, "VALIDATION_ERROR", "bad", { challenges: "Too long" });
    const { user } = setup({ onSubmit: vi.fn().mockRejectedValue(err) });
    await user.click(screen.getByRole("button", { name: "Save report" }));
    await screen.findByText("Too long");
    expect(screen.getByLabelText("Challenges")).toHaveAccessibleDescription(
      "Too long What challenges or difficulties did you encounter this month?",
    );
  });

  it("shows the server's previous-month 422 on the month field", async () => {
    const msg = "Reports can only be created for the previous month.";
    const err = new ApiError(422, "VALIDATION_ERROR", "validation failed", { month: msg });
    const { user } = setup({ lockedPeriod: undefined, onSubmit: vi.fn().mockRejectedValue(err) });
    await user.click(screen.getByRole("button", { name: "Save report" }));
    expect(await screen.findByText(msg)).toBeInTheDocument();
  });

  it("counts down a 429 and holds Save until it ends", async () => {
    const err = new ApiError(429, "RATE_LIMIT", "Too many requests.", undefined, 2);
    const { user } = setup({ onSubmit: vi.fn().mockRejectedValue(err) });
    await user.click(screen.getByRole("button", { name: "Save report" }));
    expect(await screen.findByText(/Try again in 2s/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Save report" })).toBeDisabled();
    await waitFor(() => expect(screen.getByRole("button", { name: "Save report" })).toBeEnabled(), { timeout: 3500 });
  });

  it("asks before discarding edits, and cancels straight away when nothing changed", async () => {
    const confirm = vi.spyOn(window, "confirm").mockReturnValueOnce(false).mockReturnValueOnce(true);
    const { user, onCancel } = setup();
    await user.click(screen.getByRole("button", { name: "Cancel" }));
    expect(confirm).not.toHaveBeenCalled();
    expect(onCancel).toHaveBeenCalledTimes(1);

    await user.type(screen.getByLabelText("Challenges"), "x");
    await user.click(screen.getByRole("button", { name: "Cancel" }));
    expect(onCancel).toHaveBeenCalledTimes(1); // declined
    await user.click(screen.getByRole("button", { name: "Cancel" }));
    expect(onCancel).toHaveBeenCalledTimes(2);
  });

  it("warns on tab close only while there are unsaved edits", async () => {
    const { user } = setup();
    const unload = () => {
      const e = new Event("beforeunload", { cancelable: true });
      window.dispatchEvent(e);
      return e.defaultPrevented;
    };
    expect(unload()).toBe(false);
    await user.type(screen.getByLabelText("Challenges"), "x");
    expect(unload()).toBe(true);
  });

  it("is clean again once every edit is undone", async () => {
    const confirm = vi.spyOn(window, "confirm");
    const { user, onCancel } = setup();
    await user.type(metric("Worship Service", 1), "5");
    await user.type(screen.getByLabelText("Challenges"), "x");
    await user.clear(metric("Worship Service", 1));
    await user.clear(screen.getByLabelText("Challenges"));
    await user.click(screen.getByRole("button", { name: "Cancel" }));
    expect(confirm).not.toHaveBeenCalled();
    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it("read-only shows no Save, disables the fields and offers Back", () => {
    setup({ readOnly: true });
    expect(screen.queryByRole("button", { name: "Save report" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Back" })).toBeEnabled();
    expect(metric("Outreach", 1)).toBeDisabled();
  });

  describe("restricted (D-18)", () => {
    it("replaces the free text and tithes_offering with a Restricted note, and keeps the other metrics", () => {
      setup({ readOnly: true, restricted: true });
      for (const label of ["Names of New Believers", "Narrative Report", "Challenges", "Prayer Request"]) {
        expect(screen.queryByLabelText(label)).not.toBeInTheDocument();
        expect(screen.getByText(label)).toBeInTheDocument();
      }
      expect(screen.queryByLabelText("Tithes & Offering, week 1")).not.toBeInTheDocument();
      expect(screen.getByText("Tithes & Offering")).toBeInTheDocument();
      expect(screen.getAllByText("Restricted")).toHaveLength(5);
      expect(metric("Outreach", 1)).toBeDisabled();
    });

    it("changes nothing when not restricted", () => {
      setup({ readOnly: true });
      expect(screen.getByLabelText("Narrative Report")).toBeInTheDocument();
      expect(metric("Tithes & Offering", 1)).toBeInTheDocument();
      expect(screen.queryByText("Restricted")).not.toBeInTheDocument();
    });
  });

  it("counts characters against each text limit", async () => {
    const { user } = setup();
    await user.type(screen.getByLabelText("Prayer Request"), "😀😀");
    expect(screen.getByText("2 / 10000")).toBeInTheDocument();
  });

  // WCAG 3.3.1: after a failed submit, keyboard and screen reader users land on the problem.
  describe("after a failed submit", () => {
    it("focuses the first invalid field", async () => {
      const { user } = setup();
      await user.type(metric("Outreach", 2), "1.5");
      await user.type(metric("Tithes & Offering", 1), "1.234");
      await user.click(screen.getByRole("button", { name: "Save report" }));
      // Tithes is in an earlier section than Outreach, so it is first in the page.
      expect(metric("Tithes & Offering", 1)).toHaveFocus();
    });

  it("focuses the editable field a 422 names", async () => {
    const err = new ApiError(422, "VALIDATION_ERROR", "bad", { challenges: "Too long" });
      const { user } = setup({ onSubmit: vi.fn().mockRejectedValue(err) });
      await user.click(screen.getByRole("button", { name: "Save report" }));
      await screen.findByText("Too long");
    expect(screen.getByLabelText("Challenges")).toHaveFocus();
    });

    it("a 409 on the locked month focuses its message, since the select is disabled", async () => {
      const { user } = setup({ onSubmit: vi.fn().mockRejectedValue(new ApiError(409, "CONFLICT", "exists")) });
      await user.click(screen.getByRole("button", { name: "Save report" }));
      expect(await screen.findByText("You already have a report for this month and year.")).toHaveFocus();
    });
  });

  it("links a metric's error message to its input", async () => {
    const { user } = setup();
    await user.type(metric("Outreach", 2), "1.5");
    await user.tab();
    const input = metric("Outreach", 2);
    const id = input.getAttribute("aria-describedby");
    expect(id).toBeTruthy();
    expect(document.getElementById(id!)).toHaveTextContent("Must be a whole number");
  });
});
