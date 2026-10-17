import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/api";
import { UserForm } from "./user-form";

const initial = { email: "ana@x.com", name: "Ana", password: "Str0ng!Passw0rd", confirm_password: "Str0ng!Passw0rd", role_id: "", area_assignment: "", church_name: "" };

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

describe("UserForm password confirmation", () => {
  const submit = (mode: "create" | "edit", onSubmit = vi.fn().mockResolvedValue(undefined)) => {
    render(<UserForm mode={mode} initial={initial} roles={[]} submitLabel="Save" onSubmit={onSubmit} onCancel={vi.fn()} />);
    return { user: userEvent.setup(), onSubmit };
  };

  it("create asks for the password twice, edit has neither field", () => {
    const { unmount } = render(<UserForm mode="create" initial={initial} roles={[]} submitLabel="Save" onSubmit={vi.fn()} onCancel={vi.fn()} />);
    expect(screen.getByLabelText("Password")).toBeInTheDocument();
    expect(screen.getByLabelText("Confirm password")).toBeInTheDocument();
    unmount();
    render(<UserForm mode="edit" initial={initial} roles={[]} submitLabel="Save" onSubmit={vi.fn()} onCancel={vi.fn()} />);
    expect(screen.queryByLabelText("Confirm password")).not.toBeInTheDocument();
  });

  it("an empty confirmation is a required field, so the browser stops it and nothing is sent", async () => {
    const { user, onSubmit } = submit("create");
    await user.clear(screen.getByLabelText("Confirm password"));
    await user.click(screen.getByRole("button", { name: "Save" }));
    expect(screen.getByLabelText("Confirm password")).toBeRequired();
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("a different confirmation says so and nothing is sent", async () => {
    const { user, onSubmit } = submit("create");
    await user.type(screen.getByLabelText("Confirm password"), "x");
    await user.click(screen.getByRole("button", { name: "Save" }));
    expect(onSubmit).not.toHaveBeenCalled();
    expect(await screen.findByText("Confirm password does not match")).toBeInTheDocument();
    expect(screen.getByLabelText("Confirm password")).toHaveFocus();
    expect(screen.getByLabelText("Name")).toHaveValue("Ana");
  });

  it("matching passwords submit both values", async () => {
    const { user, onSubmit } = submit("create");
    await user.click(screen.getByRole("button", { name: "Save" }));
    await vi.waitFor(() => expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({ password: "Str0ng!Passw0rd", confirm_password: "Str0ng!Passw0rd" })));
  });

  it("a server 422 on confirm_password shows on that field, and typing clears it", async () => {
    const onSubmit = vi.fn().mockRejectedValue(new ApiError(422, "VALIDATION_ERROR", "bad", { confirm_password: "does not match" }));
    const { user } = submit("create", onSubmit);
    await user.click(screen.getByRole("button", { name: "Save" }));
    expect(await screen.findByText("Confirm password does not match")).toBeInTheDocument();
    await user.type(screen.getByLabelText("Confirm password"), "y");
    expect(screen.queryByText("Confirm password does not match")).not.toBeInTheDocument();
  });

  it("edit mode submits without a confirmation", async () => {
    const { user, onSubmit } = submit("edit");
    await user.click(screen.getByRole("button", { name: "Save" }));
    await vi.waitFor(() => expect(onSubmit).toHaveBeenCalled());
  });
});
