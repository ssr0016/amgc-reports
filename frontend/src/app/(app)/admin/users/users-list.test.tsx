import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { listUsers } from "@/lib/users";
import { UsersPanel } from "./users-list";

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
}));

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
      "All statuses", "Active", "Inactive", "Disciplinary", "Deceased",
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
});
