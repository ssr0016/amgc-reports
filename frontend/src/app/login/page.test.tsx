import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/api";
import LoginPage from "./page";

const h = vi.hoisted(() => ({ signIn: vi.fn(), replace: vi.fn(), status: "unauthenticated", isAdmin: false }));
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace: h.replace }) }));
vi.mock("@/lib/session", () => ({ useSession: () => h }));

beforeEach(() => {
  vi.clearAllMocks();
  h.status = "unauthenticated";
  h.isAdmin = false;
  window.history.replaceState(null, "", "/login");
});

async function submitWith(err: ApiError) {
  h.signIn.mockRejectedValueOnce(err);
  render(<LoginPage />);
  fireEvent.change(screen.getByLabelText("Email"), { target: { value: "a@b.c" } });
  fireEvent.change(screen.getByLabelText("Password", { exact: true }), { target: { value: "x" } });
  fireEvent.click(screen.getByRole("button", { name: "Sign in" }));
}

describe("login page", () => {
  it.each([
    [true, "", "/admin"],
    [false, "", "/reports"],
    [true, "?next=/reports", "/reports"],
    [true, "?next=//evil.example", "/admin"],
  ])("redirects with admin=%s and query=%s to %s", (isAdmin, query, destination) => {
    h.status = "authenticated";
    h.isAdmin = isAdmin;
    window.history.replaceState(null, "", `/login${query}`);
    render(<LoginPage />);
    expect(h.replace).toHaveBeenCalledWith(destination);
  });

  // The page title text was a plain div, so the page had no heading at all (WCAG 1.3.1, 2.4.6).
  it("has a level-one heading", () => {
    render(<LoginPage />);
    expect(screen.getByRole("heading", { level: 1, name: "Sign in" })).toBeInTheDocument();
  });

  // The 403 is no longer only "locked": inactive, under review and closed accounts get their own sentence.
  it.each([
    "Your account is inactive. Contact your admin.",
    "Your account is under review. Contact your admin.",
    "This account is closed.",
    "Account locked. Try again later.",
  ])("shows the server's 403 message: %s", async (message) => {
    await submitWith(new ApiError(403, "FORBIDDEN", message));
    expect(await screen.findByText(message)).toBeInTheDocument();
  });

  it("keeps one generic text for a wrong password", async () => {
    await submitWith(new ApiError(401, "UNAUTHORIZED", "invalid email or password"));
    expect(await screen.findByText("Invalid email or password.")).toBeInTheDocument();
  });
});
