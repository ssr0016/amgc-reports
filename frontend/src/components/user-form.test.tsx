import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/api";
import { UserForm } from "./user-form";

const initial = { email: "ana@x.com", name: "Ana", password: "Str0ng!Passw0rd", role_id: "", area_assignment: "", church_name: "" };

function setup(error: ApiError) {
  const user = userEvent.setup();
  render(
    <UserForm mode="create" initial={initial} roles={[]} submitLabel="Create user" onSubmit={vi.fn().mockRejectedValue(error)} onCancel={vi.fn()} />,
  );
  return user;
}

describe("UserForm after a failed submit", () => {
  it("focuses the field the server names", async () => {
    const user = setup(new ApiError(422, "VALIDATION_ERROR", "bad", { name: "Too short" }));
    await user.click(screen.getByRole("button", { name: "Create user" }));
    await screen.findByText("Too short");
    expect(screen.getByLabelText("Name")).toHaveFocus();
  });

  it("a taken email focuses the email field", async () => {
    const user = setup(new ApiError(409, "CONFLICT", "exists"));
    await user.click(screen.getByRole("button", { name: "Create user" }));
    expect(await screen.findByText("That email is already in use.")).toBeInTheDocument();
    expect(screen.getByLabelText("Email")).toHaveFocus();
  });
});
