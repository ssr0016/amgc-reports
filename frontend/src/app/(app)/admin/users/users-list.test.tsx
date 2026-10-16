import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { listUsers } from "@/lib/users";
import { UsersPanel } from "./users-list";

vi.mock("@/lib/session", () => ({ useSession: () => ({ isAdmin: true, user: { user_id: 1 } }) }));
vi.mock("@/lib/users", () => ({
  listUsers: vi.fn().mockResolvedValue([
    { id: 1, name: "Ana Cruz", email: "ana@x.com", role_id: 1 },
    { id: 2, name: "Ben Reyes", email: "ben@x.com", role_id: 2 },
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
});
