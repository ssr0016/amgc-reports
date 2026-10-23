import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/api";
import { deleteRole } from "@/lib/roles";
import { listRoles, listUsers } from "@/lib/users";
import { RolesPanel } from "./roles";

const toast = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }));
const session = vi.hoisted(() => ({ isAdmin: true, perms: ["roles:write"] as string[] }));
vi.mock("sonner", () => ({ toast }));
vi.mock("@/lib/session", () => ({
  useSession: () => ({ isAdmin: session.isAdmin, can: (p: string) => session.perms.includes(p) }),
}));
vi.mock("@/lib/users", () => ({ listRoles: vi.fn(), listUsers: vi.fn() }));
vi.mock("@/lib/roles", async (orig) => ({ ...(await orig<typeof import("@/lib/roles")>()), deleteRole: vi.fn() }));

const roles = [
  { id: 1, name: "admin", description: "Everything", user_count: 2, permissions: [{ id: 1 }, { id: 2 }] },
  { id: 4, name: "Reviewer", description: "", user_count: 3, permissions: [{ id: 1 }] },
  { id: 5, name: "Unused", description: "Spare", user_count: 0 },
];

// inMain wraps the panel in the app's focusable #main-content, the dialog's focus fallback.
const wrap = (inMain = false) => {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const invalidate = vi.spyOn(client, "invalidateQueries");
  const panel = <RolesPanel />;
  render(
    <QueryClientProvider client={client}>
      {inMain ? (
        <main id="main-content" tabIndex={-1}>
          {panel}
        </main>
      ) : (
        panel
      )}
    </QueryClientProvider>,
  );
  return { invalidate };
};
const row = async (name: string) => (await screen.findByRole("cell", { name: new RegExp(`^${name}`) })).closest("tr")!;

describe("Roles tab", () => {
  beforeEach(() => {
    session.isAdmin = true;
    session.perms = ["roles:write"];
    vi.mocked(listRoles).mockReset().mockResolvedValue(roles as never);
    vi.mocked(deleteRole).mockReset();
    vi.mocked(listUsers).mockReset();
  });

  it("loading is a status message", () => {
    wrap();
    expect(screen.getByRole("status")).toHaveTextContent("Loading…");
  });

  it("a failed load shows Retry, which asks again", async () => {
    vi.mocked(listRoles).mockRejectedValueOnce(new Error("Server error."));
    wrap();
    expect(await screen.findByRole("alert")).toHaveTextContent("Server error.");
    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    await waitFor(() => expect(screen.queryByRole("alert")).not.toBeInTheDocument());
    expect(listRoles).toHaveBeenCalledTimes(2);
  });

  it("a 403 says the admin may not manage roles", async () => {
    vi.mocked(listRoles).mockRejectedValueOnce(new ApiError(403, "FORBIDDEN", "forbidden"));
    wrap();
    expect(await screen.findByText("You are not allowed to manage roles.")).toBeInTheDocument();
  });

  it("shows name, description, users and permission count, with a Built-in badge", async () => {
    wrap();
    const admin = await row("admin");
    expect(within(admin).getByText("Built-in")).toBeInTheDocument();
    expect(within(admin).getAllByRole("cell").map((c) => c.textContent)).toEqual(
      expect.arrayContaining(["Everything", "2"]),
    );
    const unused = await row("Unused");
    expect(within(unused).queryByText("Built-in")).not.toBeInTheDocument();
    expect(within(unused).getAllByRole("cell")[3]).toHaveTextContent("0");
  });

  it("hides New, Edit and Delete without roles:write", async () => {
    session.perms = [];
    wrap();
    await row("Reviewer");
    expect(screen.queryByRole("link", { name: "New role" })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /Edit/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Delete/ })).not.toBeInTheDocument();
  });

  it("a built-in role is View only, with no Delete", async () => {
    wrap();
    const admin = await row("admin");
    expect(within(admin).getByRole("link", { name: "View admin" })).toHaveAttribute("href", "/admin/roles/1/edit");
    expect(within(admin).queryByRole("button", { name: /Delete/ })).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "New role" })).toHaveAttribute("href", "/admin/roles/new");
  });

  it("a role in use needs a role to move its users to before Delete is allowed", async () => {
    vi.mocked(deleteRole).mockResolvedValueOnce(undefined);
    const { invalidate } = wrap();
    fireEvent.click(within(await row("Reviewer")).getByRole("button", { name: "Delete Reviewer" }));
    const dialog = await screen.findByRole("alertdialog");
    expect(dialog).toHaveTextContent("3 users hold this role");
    const confirm = within(dialog).getByRole("button", { name: "Move users and delete" });
    expect(confirm).toBeDisabled();

    const select = within(dialog).getByLabelText("Move 3 users to");
    const options = within(select).getAllByRole("option").map((o) => o.textContent);
    expect(options).toEqual(["Choose a role…", "Unused"]);
    fireEvent.change(select, { target: { value: "5" } });
    fireEvent.click(confirm);

    await waitFor(() => expect(deleteRole).toHaveBeenCalledWith(4, 5));
    await waitFor(() => expect(toast.success).toHaveBeenCalledWith("Moved 3 users to Unused and deleted role Reviewer"));
    expect(invalidate).toHaveBeenCalledWith({ queryKey: ["users"] });
  });

  it("the user count opens the role's users, deleted ones marked", async () => {
    vi.mocked(listUsers).mockImplementation(async (_e, status) =>
      (status === "deleted"
        ? [{ id: 9, name: "Gone", email: "gone@x.com", deleted_at: "2026-01-01T00:00:00Z" }]
        : [{ id: 8, name: "Ann", email: "ann@x.com" }]) as never,
    );
    wrap();
    expect(within(await row("Unused")).queryByRole("button", { name: /Show/ })).not.toBeInTheDocument();
    fireEvent.click(within(await row("Reviewer")).getByRole("button", { name: "Show the 3 users with role Reviewer" }));
    const dialog = await screen.findByRole("alertdialog");
    expect(await within(dialog).findByText("ann@x.com")).toBeInTheDocument();
    expect(within(dialog).getByText("Gone").closest("li")).toHaveTextContent("Deleted");
    expect(listUsers).toHaveBeenCalledWith("", "", 4);
    expect(listUsers).toHaveBeenCalledWith("", "deleted", 4);
  });

  it("deletes an unused role after confirming, and refreshes the roles", async () => {
    vi.mocked(deleteRole).mockResolvedValueOnce(undefined);
    const { invalidate } = wrap();
    fireEvent.click(within(await row("Unused")).getByRole("button", { name: "Delete Unused" }));
    const dialog = await screen.findByRole("alertdialog");
    fireEvent.click(within(dialog).getByRole("button", { name: "Delete role" }));

    await waitFor(() => expect(deleteRole).toHaveBeenCalledWith(5, undefined));
    await waitFor(() => expect(toast.success).toHaveBeenCalledWith("Deleted role Unused"));
    expect(invalidate).toHaveBeenCalledWith({ queryKey: ["roles"] });
  });

  it("shows a refused delete in the dialog", async () => {
    vi.mocked(deleteRole).mockRejectedValueOnce(
      new ApiError(409, "CONFLICT", "role is assigned to 1 users; reassign them first"),
    );
    wrap();
    fireEvent.click(within(await row("Unused")).getByRole("button", { name: "Delete Unused" }));
    const dialog = await screen.findByRole("alertdialog");
    fireEvent.click(within(dialog).getByRole("button", { name: "Delete role" }));
    expect(await within(dialog).findByRole("alert")).toHaveTextContent("reassign them first");
  });

  // WCAG 2.4.3: the dialog has no trigger element, so without help focus falls to the page body.
  it("Escape closes the dialog and gives focus back to the Delete button that opened it", async () => {
    const user = userEvent.setup();
    wrap();
    const open = within(await row("Unused")).getByRole("button", { name: "Delete Unused" });
    await user.click(open);
    await screen.findByRole("alertdialog");
    await user.keyboard("{Escape}");
    await waitFor(() => expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument());
    expect(open).toHaveFocus();
  });

  it("after a delete the row is gone, so focus goes to the main content", async () => {
    const user = userEvent.setup();
    vi.mocked(deleteRole).mockResolvedValueOnce(undefined);
    vi.mocked(listRoles)
      .mockResolvedValueOnce(roles as never)
      .mockResolvedValue(roles.filter((r) => r.name !== "Unused") as never);
    wrap(true);
    await user.click(within(await row("Unused")).getByRole("button", { name: "Delete Unused" }));
    await user.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: "Delete role" }));
    await waitFor(() => expect(screen.queryByText("Unused")).not.toBeInTheDocument());
    await waitFor(() => expect(screen.getByRole("main")).toHaveFocus());
  });
});
