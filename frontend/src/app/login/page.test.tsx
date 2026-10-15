import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import LoginPage from "./page";

vi.mock("next/navigation", () => ({ useRouter: () => ({ replace: vi.fn() }) }));
vi.mock("@/lib/session", () => ({ useSession: () => ({ signIn: vi.fn() }) }));

describe("login page", () => {
  // The page title text was a plain div, so the page had no heading at all (WCAG 1.3.1, 2.4.6).
  it("has a level-one heading", () => {
    render(<LoginPage />);
    expect(screen.getByRole("heading", { level: 1, name: "Sign in" })).toBeInTheDocument();
  });
});
