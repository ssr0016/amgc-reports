import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/api";
import { getRole, updateRole } from "@/lib/roles";
import { EditRole } from "./edit-role";

const h = vi.hoisted(() => ({ replace: vi.fn(), toast: { success: vi.fn(), error: vi.fn() } }));
vi.mock("next/navigation", () => ({
  useParams: () => ({ id: "4" }),
  useRouter: () => ({ replace: h.replace, push: vi.fn() }),
}));
vi.mock("sonner", () => ({ toast: h.toast }));
vi.mock("@/lib/session", () => ({ useSession: () => ({ isAdmin: true, can: () => true }) }));
vi.mock("@/lib/roles", async (orig) => ({
  ...(await orig<typeof import("@/lib/roles")>()),
  listPermissions: vi.fn(async () => [
    { id: 11, name: "reports:read_any", resource: "reports", action: "read_any" },
    { id: 12, name: "reports:approve", resource: "reports", action: "approve" },
    { id: 21, name: "users:read", resource: "users", action: "read" },
  ]),
  getRole: vi.fn(),
  updateRole: vi.fn(),
}));

const reviewer = { id: 4, name: "Reviewer", description: "Reads", user_count: 1, permissions: [{ id: 11 }, { id: 12 }] };

function renderEdit() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <EditRole />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(getRole).mockResolvedValue(reviewer as never);
});

describe("Edit role", () => {
  it("starts from the role's checked permissions", async () => {
    renderEdit();
    expect(await screen.findByRole("checkbox", { name: /reports:read_any/ })).toBeChecked();
    expect(screen.getByRole("checkbox", { name: /reports:approve/ })).toBeChecked();
    expect(screen.getByRole("checkbox", { name: /users:read/ })).not.toBeChecked();
  });

  it("a rename with permission changes sends the full set", async () => {
    vi.mocked(updateRole).mockResolvedValueOnce({ id: 4, name: "Senior Reviewer" } as never);
    renderEdit();
    fireEvent.change(await screen.findByLabelText("Name"), { target: { value: "Senior Reviewer" } });
    fireEvent.click(screen.getByRole("checkbox", { name: /reports:approve/ }));
    fireEvent.click(screen.getByRole("checkbox", { name: /users:read/ }));
    fireEvent.click(screen.getByRole("button", { name: "Save changes" }));

    await waitFor(() =>
      expect(updateRole).toHaveBeenCalledWith(4, { name: "Senior Reviewer", description: "Reads", permission_ids: [11, 21] }),
    );
    await waitFor(() => expect(h.replace).toHaveBeenCalledWith("/admin?tab=roles"));
    expect(h.toast.success).toHaveBeenCalledWith("Saved role Senior Reviewer");
  });

  it("an unknown permission (422) is shown under the checklist and focused", async () => {
    vi.mocked(updateRole).mockRejectedValueOnce(
      new ApiError(422, "VALIDATION", "validation failed", { permission_ids: "unknown permission" }),
    );
    renderEdit();
    fireEvent.click(await screen.findByRole("button", { name: "Save changes" }));
    const note = await screen.findByText("Permissions: unknown permission");
    await waitFor(() => expect(note).toHaveFocus());
  });

  it("a built-in role opens read-only, with a note and no Save", async () => {
    vi.mocked(getRole).mockResolvedValue({ ...reviewer, name: "admin" } as never);
    renderEdit();
    expect(await screen.findByText(/built-in role/)).toBeInTheDocument();
    expect(screen.getByLabelText("Name")).toBeDisabled();
    for (const box of screen.getAllByRole("checkbox")) expect(box).toBeDisabled();
    expect(screen.queryByRole("button", { name: "Save changes" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Back" })).toBeInTheDocument();
  });

  it("a missing role says so", async () => {
    vi.mocked(getRole).mockRejectedValue(new ApiError(404, "NOT_FOUND", "role not found"));
    renderEdit();
    expect(await screen.findByText("Role not found.")).toBeInTheDocument();
  });
});
