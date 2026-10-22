import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { ErrorRetry } from "./error-retry";
import { Loading } from "./loading";

describe("Loading", () => {
  it("announces its text, with a hidden spinner", () => {
    render(<Loading>Loading preview…</Loading>);
    const status = screen.getByRole("status");
    expect(status).toHaveTextContent("Loading preview…");
    expect(status.querySelector("svg")).toHaveAttribute("aria-hidden", "true");
  });

  it("says Loading… by default", () => {
    render(<Loading />);
    expect(screen.getByRole("status")).toHaveTextContent("Loading…");
  });
});

describe("ErrorRetry", () => {
  it("shows the message as an alert and retries on click", async () => {
    const onRetry = vi.fn();
    render(<ErrorRetry message="Server error." onRetry={onRetry} />);
    expect(screen.getByRole("alert")).toHaveTextContent("Server error.");
    await userEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(onRetry).toHaveBeenCalledTimes(1);
  });
});
