import { fileURLToPath } from "node:url";
import { defineConfig } from "vitest/config";

export default defineConfig({
  resolve: { alias: { "@": fileURLToPath(new URL("./src", import.meta.url)) } },
  test: {
    environment: "jsdom",
    include: ["src/**/*.test.{ts,tsx}"],
    setupFiles: ["./vitest.setup.ts"],
    coverage: {
      provider: "v8",
      // Logic lives in src/lib; the gate stops there so tests don't chase shadcn wrappers and page shells.
      include: ["src/lib/**"],
      exclude: ["src/lib/api-schema.ts", "src/lib/api-types.ts", "src/lib/**/*.test.*"],
      thresholds: { lines: 80, statements: 80, functions: 80, branches: 80 },
    },
  },
});
