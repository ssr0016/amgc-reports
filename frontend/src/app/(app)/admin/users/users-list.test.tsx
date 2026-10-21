import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/api";
import { deleteUser, listUsers, restoreUser } from "@/lib/users";
import { UsersPanel } from "./users-list";

const toast = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }));
vi.mock("sonner", () => ({ toast }));
vi.mock("@/lib/session", () => ({ useSession: () => ({ isAdmin: true, user: { user_id: 1 } }) }));
vi.mock("@/lib/users", () => ({
  listUsers: vi.fn().mockResolvedValue([
    { id: 1, name: "Ana Cruz", email: "ana@x.com", role_id: 1, status: "active" },
    { id: 2, name: "Ben Reyes", email: "ben@x.com", role_id: 2, status: "disciplinary" },
  ]),
  listRoles: vi.fn().mockResolvedValue([
    { id: 1, name: "user" },
    { id: 2, name: "admin" },
  ]),
  deleteUser: vi.fn(),
  restoreUser: vi.fn(),
}));

const deleted = { id: 3, name: "Cy Lim", email: "cy@x.com", role_id: 1, status: "active", deleted_at: "2026-10-21T00:00:00Z" };

const wrap = (ui: React.ReactNode) => render(<QueryClientProvider client={new QueryClient()}>{ui}</QueryClientProvider>);

describe("users list", () => {
  it("each Edit link names its user", async () => {
    wrap(<UsersPanel />);
    expect(await screen.findByRole("link", { name: "Edit Ana Cruz" })).toHaveAttribute("href", "/admin/users/1/edit");
    expect(screen.getByRole("link", { name: "Edit Ben Reyes" })).toBeInTheDocument();
  });

  it("loading is a status message", () => {
    wrap(<UsersPanel />);
    expect(screen.getByRole("status")).toHaveTextContent("Loading…");
  });

  it("marks only the signed-in user's row with a You badge", async () => {
    wrap(<UsersPanel />);
    const ana = (await screen.findByRole("link", { name: "Edit Ana Cruz" })).closest("tr")!;
    const ben = screen.getByRole("link", { name: "Edit Ben Reyes" }).closest("tr")!;
    expect(within(ana).getByText("You")).toBeInTheDocument();
    expect(within(ben).queryByText("You")).not.toBeInTheDocument();
  });

  it("the role filter narrows the loaded rows", async () => {
    wrap(<UsersPanel />);
    await screen.findByRole("link", { name: "Edit Ana Cruz" });
    fireEvent.change(screen.getByLabelText("Role"), { target: { value: "2" } });
    expect(screen.queryByRole("link", { name: "Edit Ana Cruz" })).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Edit Ben Reyes" })).toBeInTheDocument();
  });

  it("a role with no loaded users says so", async () => {
    vi.mocked(listUsers).mockResolvedValueOnce([{ id: 1, name: "Ana Cruz", email: "ana@x.com", role_id: 1 }] as never);
    wrap(<UsersPanel />);
    await screen.findByRole("link", { name: "Edit Ana Cruz" });
    fireEvent.change(screen.getByLabelText("Role"), { target: { value: "2" } });
    expect(await screen.findByText("No users match.")).toBeInTheDocument();
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
    expect(vi.mocked(listUsers)).toHaveBeenLastCalledWith("", "");
    const filter = screen.getByLabelText("Status");
    expect([...(filter as HTMLSelectElement).options].map((o) => o.text)).toEqual([
      "All statuses", "Active", "Inactive", "Disciplinary", "Deceased", "Deleted",
    ]);

    fireEvent.change(filter, { target: { value: "inactive" } });
    await waitFor(() => expect(vi.mocked(listUsers)).toHaveBeenLastCalledWith("", "inactive"));
    // Back to All statuses is served from the query cache, so no new request is needed.
    fireEvent.change(filter, { target: { value: "" } });
    expect((filter as HTMLSelectElement).value).toBe("");
    expect(await screen.findByRole("link", { name: "Edit Ana Cruz" })).toBeInTheDocument();
  });

  it("the Role filter stays client-side: it does not ask the server again", async () => {
    wrap(<UsersPanel />);
    await screen.findByRole("link", { name: "Edit Ana Cruz" });
    const before = vi.mocked(listUsers).mock.calls.length;
    fireEvent.change(screen.getByLabelText("Role"), { target: { value: "2" } });
    expect(vi.mocked(listUsers).mock.calls.length).toBe(before);
  });

  it("the Deleted option asks the server for status=deleted", async () => {
    wrap(<UsersPanel />);
    await screen.findByRole("link", { name: "Edit Ana Cruz" });
    fireEvent.change(screen.getByLabelText("Status"), { target: { value: "deleted" } });
    await waitFor(() => expect(vi.mocked(listUsers)).toHaveBeenLastCalledWith("", "deleted"));
  });

  it("a deleted row shows the Deleted badge and Restore, not Delete", async () => {
    vi.mocked(listUsers).mockResolvedValueOnce([deleted] as never);
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
    wrap(<UsersPanel />);
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
  });

  it("a 409 stays in the dialog as an alert", async () => {
    vi.mocked(deleteUser).mockRejectedValueOnce(new ApiError(409, "CONFLICT", "At least one active admin is required."));
    wrap(<UsersPanel />);
    fireEvent.click(await screen.findByRole("button", { name: "Delete Ben Reyes" }));
    const dialog = await screen.findByRole("alertdialog");
    fireEvent.click(within(dialog).getByRole("button", { name: "Delete user" }));
    expect(await within(dialog).findByRole("alert")).toHaveTextContent("At least one active admin is required.");
    expect(screen.getByRole("alertdialog")).toBeInTheDocument();
  });

  it("Restore calls the API and toasts", async () => {
    vi.mocked(listUsers).mockResolvedValueOnce([deleted] as never);
    vi.mocked(restoreUser).mockResolvedValueOnce({} as never);
    wrap(<UsersPanel />);
    fireEvent.click(await screen.findByRole("button", { name: "Restore Cy Lim" }));
    await waitFor(() => expect(toast.success).toHaveBeenCalledWith("Cy Lim restored"));
    expect(vi.mocked(restoreUser)).toHaveBeenCalledWith(3);
  });
});
