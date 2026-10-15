import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/api";
import { emptyForm, type FormState } from "@/lib/report-form";
import { ReportForm } from "./report-form";

const period = { month: "April", year: 2026 };
const initial = (): FormState => ({ ...emptyForm({ worker: "Ana" }), month: "April", year: "2026" });

function setup(props: Partial<React.ComponentProps<typeof ReportForm>> = {}) {
  const onSubmit = vi.fn().mockResolvedValue(undefined);
  const onCancel = vi.fn();
  const user = userEvent.setup();
  render(
    <ReportForm initial={initial()} submitLabel="Save report" lockedPeriod={period} onSubmit={onSubmit} onCancel={onCancel} {...props} />,
  );
  return { user, onSubmit, onCancel };
}

const metric = (label: string, week: number) => screen.getByLabelText(`${label}, week ${week}`);

afterEach(() => vi.restoreAllMocks());

describe("ReportForm", () => {
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
    const err = new ApiError(422, "VALIDATION_ERROR", "bad", { worker: "Too long", "metrics[outreach]": "Bad metric" });
    const { user } = setup({ onSubmit: vi.fn().mockRejectedValue(err) });
    await user.click(screen.getByRole("button", { name: "Save report" }));
    expect(await screen.findByText("Too long")).toBeInTheDocument();
    expect(screen.getByText("Bad metric")).toBeInTheDocument();
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

  it("read-only shows no Save, disables the fields and offers Back", () => {
    setup({ readOnly: true });
    expect(screen.queryByRole("button", { name: "Save report" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Back" })).toBeEnabled();
    expect(metric("Outreach", 1)).toBeDisabled();
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

    it("focuses the field a 422 names", async () => {
      const err = new ApiError(422, "VALIDATION_ERROR", "bad", { worker: "Too long" });
      const { user } = setup({ onSubmit: vi.fn().mockRejectedValue(err) });
      await user.click(screen.getByRole("button", { name: "Save report" }));
      await screen.findByText("Too long");
      expect(screen.getByLabelText("Worker Name")).toHaveFocus();
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
