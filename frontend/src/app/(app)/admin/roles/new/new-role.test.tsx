import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/api";
import { createRole } from "@/lib/roles";
import { NewRole } from "./new-role";

const h = vi.hoisted(() => ({ replace: vi.fn(), push: vi.fn(), toast: { success: vi.fn(), error: vi.fn() } }));
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace: h.replace, push: h.push }) }));
vi.mock("sonner", () => ({ toast: h.toast }));
vi.mock("@/lib/session", () => ({ useSession: () => ({ isAdmin: true, can: () => true }) }));
vi.mock("@/lib/roles", async (orig) => ({
  ...(await orig<typeof import("@/lib/roles")>()),
  listPermissions: vi.fn(async () => [
    { id: 11, name: "reports:read_any", resource: "reports", action: "read_any" },
    { id: 12, name: "reports:approve", resource: "reports", action: "approve" },
    { id: 21, name: "users:read", resource: "users", action: "read" },
    { id: 31, name: "roles:write", resource: "roles", action: "write" },
  ]),
  createRole: vi.fn(),
}));

function renderNew() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const invalidate = vi.spyOn(client, "invalidateQueries");
  render(
    <QueryClientProvider client={client}>
      <NewRole />
    </QueryClientProvider>,
  );
  return { invalidate };
}

beforeEach(() => vi.clearAllMocks());

describe("New role", () => {
  it("groups the checklist under Reports, Users and Admin, each checkbox labelled in words", async () => {
    renderNew();
    const reports = await screen.findByRole("group", { name: "Reports" });
    expect(within(reports).getByRole("checkbox", { name: /View anyone's.*reports:read_any/ })).toBeInTheDocument();
    expect(screen.getAllByRole("group").map((g) => g.querySelector("legend")?.textContent)).toEqual([
      "Reports",
      "Users",
      "Admin",
    ]);
  });

  it("creates the role with the checked permissions, then returns to the Roles tab", async () => {
    vi.mocked(createRole).mockResolvedValueOnce({ id: 4, name: "Reviewer" } as never);
    const { invalidate } = renderNew();
    fireEvent.change(await screen.findByLabelText("Name"), { target: { value: " Reviewer " } });
    fireEvent.change(screen.getByLabelText("Description"), { target: { value: "Reads reports" } });
    fireEvent.click(screen.getByRole("checkbox", { name: /reports:read_any/ }));
    fireEvent.click(screen.getByRole("checkbox", { name: /reports:approve/ }));
    fireEvent.click(screen.getByRole("button", { name: "Create role" }));

    await waitFor(() =>
      expect(createRole).toHaveBeenCalledWith({ name: "Reviewer", description: "Reads reports", permission_ids: [11, 12] }),
    );
    await waitFor(() => expect(h.replace).toHaveBeenCalledWith("/admin?tab=roles"));
    expect(h.toast.success).toHaveBeenCalledWith("Created role Reviewer");
    expect(invalidate).toHaveBeenCalledWith({ queryKey: ["roles"] });
  });

  it("a duplicate name shows the server's message as an alert", async () => {
    vi.mocked(createRole).mockRejectedValueOnce(new ApiError(409, "CONFLICT", "role name already exists"));
    renderNew();
    fireEvent.change(await screen.findByLabelText("Name"), { target: { value: "admin" } });
    fireEvent.click(screen.getByRole("button", { name: "Create role" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("role name already exists");
    expect(h.replace).not.toHaveBeenCalled();
  });

  it("a 422 marks the field, describes it with the error, and focuses it", async () => {
    vi.mocked(createRole).mockRejectedValueOnce(
      new ApiError(422, "VALIDATION", "validation failed", { name: "must be at least 2 characters" }),
    );
    renderNew();
    const name = await screen.findByLabelText("Name");
    fireEvent.change(name, { target: { value: "xy" } });
    fireEvent.click(screen.getByRole("button", { name: "Create role" }));

    await waitFor(() => expect(name).toHaveAttribute("aria-invalid", "true"));
    expect(name).toHaveAccessibleDescription("must be at least 2 characters");
    await waitFor(() => expect(name).toHaveFocus());
  });

  it("Cancel goes back to the Roles tab", async () => {
    renderNew();
    fireEvent.click(await screen.findByRole("button", { name: "Cancel" }));
    expect(h.push).toHaveBeenCalledWith("/admin?tab=roles");
  });
});
