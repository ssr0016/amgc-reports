import { defineConfig } from "@playwright/test";

// One worker: the specs share a database, and the per-user write limits are easier
// to reason about when tests do not run in parallel.
export default defineConfig({
  testDir: "./e2e",
  fullyParallel: false,
  workers: 1,
  retries: 0,
  forbidOnly: !!process.env.CI,
  reporter: [["list"], ["html", { open: "never" }]],
  use: { baseURL: "http://localhost:3001", trace: "retain-on-failure" },
  projects: [
    { name: "setup", testMatch: /setup\.ts/ },
    { name: "e2e", testMatch: /.*\.spec\.ts/, dependencies: ["setup"] },
  ],
  webServer: [
    {
      command: "./e2e/start-backend.sh",
      url: "http://localhost:8081/health",
      timeout: 180_000,
      reuseExistingServer: false,
    },
    {
      // A stale .next-e2e cache once served CSS from before an edit, so start from nothing.
      command: "rm -rf .next-e2e && next dev -p 3001",
      url: "http://localhost:3001/login",
      timeout: 120_000,
      reuseExistingServer: false,
      env: { NEXT_DIST_DIR: ".next-e2e", NEXT_PUBLIC_API_URL: "http://localhost:8081/api/v1" },
    },
  ],
});
