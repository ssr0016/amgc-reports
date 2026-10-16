import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import AppLayout from "./layout";

const h = vi.hoisted(() => ({ status: "authenticated" }));
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace: vi.fn() }) }));
vi.mock("@/lib/session", () => ({
  useSession: () => ({ status: h.status, user: { name: "Ana", email: "a@b.c", role: "user" }, isAdmin: false, signOut: vi.fn() }),
}));

describe("app layout", () => {
  // WCAG 2.4.1: a way past the header on every page.
  it("starts with a skip link to the main content", () => {
    render(<AppLayout>content</AppLayout>);
    const skip = screen.getByRole("link", { name: "Skip to main content" });
    expect(skip).toHaveAttribute("href", "#main-content");
    // first in the tab order, before the logo
    expect(screen.getAllByRole("link")[0]).toBe(skip);
  });

  it("does not show the Admin link to a non-admin", () => {
    render(<AppLayout>content</AppLayout>);
    expect(screen.queryByRole("link", { name: "Admin" })).not.toBeInTheDocument();
  });

  it("shows no header, and so no skip link, while the session loads", () => {
    h.status = "loading";
    render(<AppLayout>content</AppLayout>);
    expect(screen.queryByRole("link", { name: "Skip to main content" })).not.toBeInTheDocument();
    expect(screen.getByRole("main")).toHaveTextContent("Loading…");
    h.status = "authenticated";
  });
});
