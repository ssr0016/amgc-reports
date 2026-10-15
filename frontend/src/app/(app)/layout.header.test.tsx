import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import AppLayout from "./layout";

vi.mock("next/navigation", () => ({ useRouter: () => ({ replace: vi.fn() }) }));
vi.mock("@/lib/session", () => ({
  useSession: () => ({ status: "authenticated", user: { name: "Ana", email: "a@b.c", role: "admin" }, isAdmin: true, signOut: vi.fn() }),
}));

describe("app header", () => {
  // WCAG 1.4.10: at 320 px the admin's row (logo, Users, name and role, Sign out) is 355 px wide.
  // jsdom has no layout, so this checks that the rows may wrap; e2e/a11y.spec.ts measures the page.
  it("lets its rows wrap instead of overflowing a narrow screen", () => {
    render(<AppLayout>content</AppLayout>);
    const header = screen.getByRole("banner");
    expect(header.firstElementChild).toHaveClass("flex-wrap");
    expect(screen.getByRole("button", { name: "Sign out" }).parentElement).toHaveClass("flex-wrap");
  });
});
