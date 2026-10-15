import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { Users } from "./users-list";

vi.mock("@/lib/session", () => ({ useSession: () => ({ isAdmin: true }) }));
vi.mock("@/lib/users", () => ({
  listUsers: vi.fn().mockResolvedValue([
    { id: 1, name: "Ana Cruz", email: "ana@x.com", role_id: 1 },
    { id: 2, name: "Ben Reyes", email: "ben@x.com", role_id: 1 },
  ]),
  listRoles: vi.fn().mockResolvedValue([{ id: 1, name: "user" }]),
}));

describe("users list", () => {
  it("each Edit link names its user, and the page has the skip link target", async () => {
    render(
      <QueryClientProvider client={new QueryClient()}>
        <Users />
      </QueryClientProvider>,
    );
    expect(await screen.findByRole("link", { name: "Edit Ana Cruz" })).toHaveAttribute("href", "/admin/users/1/edit");
    expect(screen.getByRole("link", { name: "Edit Ben Reyes" })).toBeInTheDocument();
    expect(screen.getByRole("main")).toHaveAttribute("id", "main-content");
  });
});
