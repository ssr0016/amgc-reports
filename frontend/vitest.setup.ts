import "@testing-library/jest-dom/vitest";
import { cleanup } from "@testing-library/react";
import { afterEach, vi } from "vitest";

// Next's build compiler supplies these font loaders; Vitest imports pages directly.
vi.mock("next/font/google", () => ({
  Inter: () => ({ className: "inter" }),
  JetBrains_Mono: () => ({ variable: "mono" }),
}));

// Testing Library only cleans up on its own when the runner exposes globals.
afterEach(cleanup);
