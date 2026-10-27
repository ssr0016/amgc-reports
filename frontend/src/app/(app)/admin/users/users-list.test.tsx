import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/api";
import { deleteUser, listRoles, listUsers, restoreUser } from "@/lib/users";
import { UsersPanel } from "./users-list";

const toast = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }));
vi.mock("sonner", () => ({ toast }));
const session = vi.hoisted(() => ({ isAdmin: true, perms: ["users:list"] as string[] }));
vi.mock("@/lib/session", () => ({
  useSession: () => ({ isAdmin: session.isAdmin, can: (p: string) => session.perms.includes(p), user: { user_id: 1 } }),
}));
// One page of users as the API answers it.
const page = vi.hoisted(() => (data: object[], meta = {}) => ({
  data,
  meta: { page: 1, limit: 20, total: data.length, total_pages: 1, ...meta },
}));
vi.mock("@/lib/users", () => ({
  listUsers: vi.fn(),
  listRoles: vi.fn().mockResolvedValue([
    { id: 1, name: "user" },
    { id: 2, name: "admin" },
  ]),
  deleteUser: vi.fn(),
  restoreUser: vi.fn(),
}));

const deleted = { id: 3, name: "Cy Lim", email: "cy@x.com", role_id: 1, status: "active", deleted_at: "2026-10-21T00:00:00Z" };

const wrap = (ui: React.ReactNode) => {
  const client = new QueryClient();
  const invalidate = vi.spyOn(client, "invalidateQueries");
  return { ...render(<QueryClientProvider client={client}>{ui}</QueryClientProvider>), invalidate };
};

beforeEach(() => {
  vi.mocked(listUsers).mockResolvedValue(
    page([
      { id: 1, name: "Ana Cruz", email: "ana@x.com", role_id: 1, status: "active" },
      { id: 2, name: "Ben Reyes", email: "ben@x.com", role_id: 2, status: "disciplinary" },
    ]) as never,
  );
  session.isAdmin = true;
  session.perms = ["users:list"];
});

describe("users list", () => {
  it("a failed load shows Retry, which asks again", async () => {
    vi.mocked(listUsers).mockRejectedValueOnce(new Error("Server error."));
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={client}>
        <UsersPanel />
      </QueryClientProvider>,
    );
    expect(await screen.findByRole("alert")).toHaveTextContent("Server error.");
    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    await waitFor(() => expect(screen.queryByRole("alert")).not.toBeInTheDocument());
  });

  it("each Edit link names its user", async () => {
    wrap(<UsersPanel />);
    expect(await screen.findByRole("link", { name: "Edit Ana Cruz" })).toHaveAttribute("href", "/admin/users/1/edit");
    expect(screen.getByRole("link", { name: "Edit Ben Reyes" })).toBeInTheDocument();
  });

  it("loading is a status message", () => {
    wrap(<UsersPanel />);
    expect(screen.getByRole("status", { name: "Loading table" })).toBeInTheDocument();
  });

  it("marks only the signed-in user's row with a You badge", async () => {
    wrap(<UsersPanel />);
    const ana = (await screen.findByRole("link", { name: "Edit Ana Cruz" })).closest("tr")!;
    const ben = screen.getByRole("link", { name: "Edit Ben Reyes" }).closest("tr")!;
    expect(within(ana).getByText("You")).toBeInTheDocument();
    expect(within(ben).queryByText("You")).not.toBeInTheDocument();
  });

  it("an empty page says no users match", async () => {
    vi.mocked(listUsers).mockResolvedValueOnce(page([]) as never);
    wrap(<UsersPanel />);
    expect(await screen.findByText("No users match")).toBeInTheDocument();
  });

  it("Next asks the server for the next page, and a filter starts again at page 1", async () => {
    vi.mocked(listUsers).mockResolvedValue(page([deleted], { total: 45, total_pages: 3 }) as never);
    wrap(<UsersPanel />);
    await screen.findByText("Page 1 of 3");
    expect(screen.getByRole("button", { name: "Previous" })).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: "Next" }));
    await waitFor(() => expect(vi.mocked(listUsers)).toHaveBeenLastCalledWith("", "", undefined, 2, 20));
    fireEvent.change(screen.getByLabelText("Status"), { target: { value: "inactive" } });
    await waitFor(() => expect(vi.mocked(listUsers)).toHaveBeenLastCalledWith("", "inactive", undefined, 1, 20));
  });

  it("the embeddable panel has no main landmark or heading of its own", async () => {
    wrap(<UsersPanel />);
    expect(await screen.findByRole("link", { name: "Edit Ana Cruz" })).toBeInTheDocument();
    expect(screen.queryByRole("main")).not.toBeInTheDocument();
    expect(screen.queryByRole("heading", { level: 1 })).not.toBeInTheDocument();
  });

  it("shows each user's status as a badge in a Status column", async () => {
    wrap(<UsersPanel />);
    const ana = (await screen.findByRole("link", { name: "Edit Ana Cruz" })).closest("tr")!;
    const ben = screen.getByRole("link", { name: "Edit Ben Reyes" }).closest("tr")!;
    expect(screen.getByRole("columnheader", { name: "Status" })).toBeInTheDocument();
    expect(within(ana).getByText("Active")).toHaveAttribute("data-status", "active");
    expect(within(ben).getByText("Disciplinary")).toHaveAttribute("data-status", "disciplinary");
  });

  it("the Status filter asks the server, and All statuses asks for everyone", async () => {
    wrap(<UsersPanel />);
    await screen.findByRole("link", { name: "Edit Ana Cruz" });
    expect(vi.mocked(listUsers)).toHaveBeenLastCalledWith("", "", undefined, 1, 20);
    const filter = screen.getByLabelText("Status");
    expect([...(filter as HTMLSelectElement).options].map((o) => o.text)).toEqual([
      "All statuses", "Active", "Inactive", "Disciplinary", "Deceased", "Deleted",
    ]);

    fireEvent.change(filter, { target: { value: "inactive" } });
    await waitFor(() => expect(vi.mocked(listUsers)).toHaveBeenLastCalledWith("", "inactive", undefined, 1, 20));
    // Back to All statuses is served from the query cache, so no new request is needed.
    fireEvent.change(filter, { target: { value: "" } });
    expect((filter as HTMLSelectElement).value).toBe("");
    expect(await screen.findByRole("link", { name: "Edit Ana Cruz" })).toBeInTheDocument();
  });

  it("the Role filter asks the server for that role", async () => {
    wrap(<UsersPanel />);
    await screen.findByRole("link", { name: "Edit Ana Cruz" });
    fireEvent.change(screen.getByLabelText("Role"), { target: { value: "2" } });
    await waitFor(() => expect(vi.mocked(listUsers)).toHaveBeenLastCalledWith("", "", 2, 1, 20));
  });

  it("the Deleted option asks the server for status=deleted", async () => {
    wrap(<UsersPanel />);
    await screen.findByRole("link", { name: "Edit Ana Cruz" });
    fireEvent.change(screen.getByLabelText("Status"), { target: { value: "deleted" } });
    await waitFor(() => expect(vi.mocked(listUsers)).toHaveBeenLastCalledWith("", "deleted", undefined, 1, 20));
  });

  it("a deleted row shows the Deleted badge and Restore, not Delete", async () => {
    vi.mocked(listUsers).mockResolvedValueOnce(page([deleted]) as never);
    wrap(<UsersPanel />);
    const row = (await screen.findByRole("link", { name: "Edit Cy Lim" })).closest("tr")!;
    expect(within(row).getByText("Deleted")).toBeInTheDocument();
    expect(within(row).getByRole("button", { name: "Restore Cy Lim" })).toBeInTheDocument();
    expect(within(row).queryByRole("button", { name: "Delete Cy Lim" })).not.toBeInTheDocument();
  });

  it("your own row's Delete is disabled and says why", async () => {
    wrap(<UsersPanel />);
    const own = await screen.findByRole("button", { name: "Delete Ana Cruz" });
    expect(own).toBeDisabled();
    expect(own).toHaveAccessibleDescription("You cannot delete your own account.");
    expect(screen.getByRole("button", { name: "Delete Ben Reyes" })).toBeEnabled();
  });

  it("Delete confirms with the D-A11 text, sends the reason, then closes and toasts", async () => {
    vi.mocked(deleteUser).mockResolvedValueOnce({} as never);
    const { invalidate } = wrap(<UsersPanel />);
    fireEvent.click(await screen.findByRole("button", { name: "Delete Ben Reyes" }));
    const dialog = await screen.findByRole("alertdialog");
    expect(within(dialog).getByText("Delete Ben Reyes?")).toBeInTheDocument();
    expect(dialog).toHaveTextContent(
      "This will soft-delete Ben Reyes (ben@x.com). The user cannot sign in. The user is hidden from the active list, but their reports stay. You can restore them later.",
    );
    const reason = within(dialog).getByLabelText("Reason (optional)");
    expect(reason).toHaveValue("");
    expect(reason).toHaveAttribute("maxLength", "500");
    fireEvent.change(reason, { target: { value: "Left the church" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Delete user" }));
    await waitFor(() => expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument());
    expect(vi.mocked(deleteUser)).toHaveBeenCalledWith(2, "Left the church");
    expect(toast.success).toHaveBeenCalledWith("Ben Reyes deleted");
    await waitFor(() => expect(invalidate).toHaveBeenCalledWith({ queryKey: ["reports"] }));
  });

  it("a 409 stays in the dialog as an alert", async () => {
    vi.mocked(deleteUser).mockRejectedValueOnce(new ApiError(409, "CONFLICT", "At least one active admin is required."));
    const { invalidate } = wrap(<UsersPanel />);
    fireEvent.click(await screen.findByRole("button", { name: "Delete Ben Reyes" }));
    const dialog = await screen.findByRole("alertdialog");
    fireEvent.click(within(dialog).getByRole("button", { name: "Delete user" }));
    expect(await within(dialog).findByRole("alert")).toHaveTextContent("At least one active admin is required.");
    expect(screen.getByRole("alertdialog")).toBeInTheDocument();
    expect(invalidate).not.toHaveBeenCalledWith({ queryKey: ["reports"] });
  });

  it("Restore calls the API and toasts", async () => {
    vi.mocked(listUsers).mockResolvedValueOnce(page([deleted]) as never);
    vi.mocked(restoreUser).mockResolvedValueOnce({} as never);
    const { invalidate } = wrap(<UsersPanel />);
    fireEvent.click(await screen.findByRole("button", { name: "Restore Cy Lim" }));
    await waitFor(() => expect(toast.success).toHaveBeenCalledWith("Cy Lim restored"));
    expect(vi.mocked(restoreUser)).toHaveBeenCalledWith(3);
    await waitFor(() => expect(invalidate).toHaveBeenCalledWith({ queryKey: ["reports"] }));
  });
});

describe("users list for a custom role (37b)", () => {
  it("a users:list holder sees the list read-only: no Edit, Delete or New user, no role filter, no roles request", async () => {
    session.isAdmin = false;
    vi.mocked(listRoles).mockClear();
    wrap(<UsersPanel />);
    const row = (await screen.findByText("Ben Reyes")).closest("tr")!;
    expect(within(row).getByText("#2")).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /^Edit/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /^Delete/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "New user" })).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Role")).not.toBeInTheDocument();
    expect(screen.getByLabelText("Email")).toBeInTheDocument();
    expect(listRoles).not.toHaveBeenCalled();
  });

  it("without users:list it says so and never asks the API", () => {
    session.isAdmin = false;
    session.perms = ["users:read"];
    vi.mocked(listUsers).mockClear();
    wrap(<UsersPanel />);
    expect(screen.getByText("You are not allowed to manage users.")).toBeInTheDocument();
    expect(listUsers).not.toHaveBeenCalled();
  });
});
