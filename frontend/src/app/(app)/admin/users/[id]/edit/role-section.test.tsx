import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/api";
import { changeUserRole } from "@/lib/roles";
import { RoleSection } from "./role-section";

const h = vi.hoisted(() => ({ me: { user_id: 1 }, toast: { success: vi.fn(), error: vi.fn() } }));
vi.mock("sonner", () => ({ toast: h.toast }));
vi.mock("@/lib/session", () => ({ useSession: () => ({ user: h.me }) }));
vi.mock("@/lib/users", () => ({
  listRoles: vi.fn(async () => [
    { id: 1, name: "admin" },
    { id: 3, name: "user" },
    { id: 4, name: "CGP Coordinator" },
  ]),
}));
vi.mock("@/lib/roles", () => ({ changeUserRole: vi.fn() }));

function renderSection(user = { id: 7, name: "Ana Cruz", role_id: 3 }) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const invalidate = vi.spyOn(client, "invalidateQueries");
  render(
    <QueryClientProvider client={client}>
      <RoleSection user={user} />
    </QueryClientProvider>,
  );
  return { invalidate, section: () => within(screen.getByRole("group", { name: "Role" })) };
}

beforeEach(() => {
  vi.clearAllMocks();
  h.me = { user_id: 1 };
});

describe("Edit user: role", () => {
  it("lists every role, custom ones included, on the current one", async () => {
    const { section } = renderSection();
    const select = (await section().findByRole("option", { name: "CGP Coordinator" })).closest("select")!;
    expect(select).toHaveValue("3");
    expect(section().getByRole("button", { name: "Update role" })).toBeDisabled();
  });

  it("saves the chosen role, says so, and refreshes users, roles and the admin views", async () => {
    vi.mocked(changeUserRole).mockResolvedValueOnce({ id: 7, role_id: 4 } as never);
    const { section, invalidate } = renderSection();
    await section().findByRole("option", { name: "CGP Coordinator" });
    fireEvent.change(section().getByLabelText("Role"), { target: { value: "4" } });
    fireEvent.click(section().getByRole("button", { name: "Update role" }));

    await waitFor(() => expect(changeUserRole).toHaveBeenCalledWith(7, 4));
    await waitFor(() => expect(h.toast.success).toHaveBeenCalledWith("Ana Cruz now has the CGP Coordinator role"));
    for (const key of [["users"], ["user", 7], ["roles"], ["tracker"], ["admin-stats"]]) {
      expect(invalidate).toHaveBeenCalledWith({ queryKey: key });
    }
    await waitFor(() => expect(section().getByRole("button", { name: "Update role" })).toBeDisabled());
  });

  it("shows the server's 409 message", async () => {
    vi.mocked(changeUserRole).mockRejectedValueOnce(
      new ApiError(409, "CONFLICT", "At least one active admin is required."),
    );
    const { section } = renderSection({ id: 7, name: "Ana Cruz", role_id: 1 });
    await section().findByRole("option", { name: "user" });
    fireEvent.change(section().getByLabelText("Role"), { target: { value: "3" } });
    fireEvent.click(section().getByRole("button", { name: "Update role" }));
    expect(await section().findByRole("alert")).toHaveTextContent("At least one active admin is required.");
  });

  it("on your own account there is no dropdown, only a note", () => {
    const { section } = renderSection({ id: 1, name: "Me", role_id: 1 });
    expect(section().getByText("You cannot change your own role.")).toBeInTheDocument();
    expect(section().queryByRole("combobox")).not.toBeInTheDocument();
    expect(section().queryByRole("button")).not.toBeInTheDocument();
  });
});
