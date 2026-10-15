import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { AuthField } from "./auth-field";

describe("AuthField password toggle", () => {
  // A toggle button keeps one name and says its state with aria-pressed; changing the
  // label as well made screen readers say "Hide password, pressed" (WCAG 4.1.2).
  it("keeps one label and reports its state with aria-pressed", async () => {
    const user = userEvent.setup();
    render(<AuthField name="password" label="Password" type="password" />);
    const toggle = screen.getByRole("button", { name: "Show password" });
    const input = screen.getByLabelText("Password", { exact: true });
    expect(toggle).toHaveAttribute("aria-pressed", "false");
    expect(input).toHaveAttribute("type", "password");

    await user.click(toggle);
    expect(screen.getByRole("button", { name: "Show password" })).toHaveAttribute("aria-pressed", "true");
    expect(input).toHaveAttribute("type", "text");
  });

  it("links an error to its field", () => {
    render(<AuthField name="email" label="Email" error="Taken" />);
    const input = screen.getByLabelText("Email");
    expect(input).toHaveAttribute("aria-invalid", "true");
    expect(document.getElementById(input.getAttribute("aria-describedby")!)).toHaveTextContent("Taken");
  });
});
