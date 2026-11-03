import { act, render, screen } from "@testing-library/react";
import { describe, expect, it, beforeEach, vi } from "vitest";
import { ThemeProvider, useTheme } from "./theme-provider";

function Probe() {
  const { theme, toggleTheme } = useTheme();
  return <button onClick={toggleTheme}>{theme}</button>;
}

describe("ThemeProvider", () => {
  beforeEach(() => {
    localStorage.clear();
    document.documentElement.className = "";
    vi.mocked(window.matchMedia).mockReturnValue({
      matches: false,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    } as unknown as MediaQueryList);
  });

  it("uses and persists the saved theme", () => {
    localStorage.setItem("theme", "light");
    render(<ThemeProvider><Probe /></ThemeProvider>);

    expect(screen.getByRole("button")).toHaveTextContent("light");
    expect(document.documentElement).not.toHaveClass("dark");

    act(() => screen.getByRole("button").click());
    expect(screen.getByRole("button")).toHaveTextContent("dark");
    expect(localStorage.getItem("theme")).toBe("dark");
    expect(document.documentElement).toHaveClass("dark");
  });

  it("uses the light OS preference when no theme is saved", () => {
    vi.mocked(window.matchMedia).mockReturnValue({
      matches: true,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    } as unknown as MediaQueryList);
    render(<ThemeProvider><Probe /></ThemeProvider>);

    expect(screen.getByRole("button")).toHaveTextContent("light");
    expect(localStorage.getItem("theme")).toBeNull();
  });
});
