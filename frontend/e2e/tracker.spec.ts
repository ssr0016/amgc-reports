import { ACCOUNTS, API, authFile, clientIp, earlierPeriod, expect, previousPeriod, send, test } from "./support";

test.describe("as admin", () => {
  test.use({ storageState: authFile("admin") });

  test("the Tracker shows the e2e user's report as Pending, then Approved, and leaves the editor out", async ({ page, request, playwright }) => {
    // A plain user may file only the previous month; another spec may have left a report there, so start clean.
    const period = previousPeriod();
    const worker = await playwright.request.newContext({
      storageState: authFile("user"),
      extraHTTPHeaders: { "X-Forwarded-For": clientIp("tracker-user") },
    });
    const old = await (await worker.get(`${API}/reports?month=${period.month}&year=${period.year}`)).json();
    for (const r of old.data) await send(worker, "DELETE", `/reports/${r.id}`);
    const filed = await send(worker, "POST", "/reports", { ...period, worker: "E2E Tracker Worker" });
    expect(filed.status(), await filed.text()).toBe(201);
    const { id } = await filed.json();
    await worker.dispose();
    try {
      await page.goto("/admin?tab=tracker");
      const tracker = page.getByRole("region", { name: "Tracker" });
      await tracker.getByLabel("Month").selectOption(period.month);
      await tracker.getByLabel("Year").selectOption(String(period.year));
      const row = tracker.getByRole("row").filter({ hasText: ACCOUNTS.user.email });
      await expect(row).toContainText(ACCOUNTS.user.church_name);
      await expect(row).toContainText("Pending");
      await expect(tracker.getByRole("row").filter({ hasText: ACCOUNTS.editor.email })).toHaveCount(0);
      await expect(tracker.getByRole("progressbar")).toBeVisible();

      expect((await send(request, "PATCH", `/reports/${id}/revision`, { admin_notes: "Please clarify" })).status()).toBe(200);
      await page.reload();
      await tracker.getByLabel("Month").selectOption(period.month);
      await tracker.getByLabel("Year").selectOption(String(period.year));
      await expect(row).toContainText("Needs Revision");

      expect((await send(request, "PATCH", `/reports/${id}/complete`)).status()).toBe(200);
      await page.reload();
      await tracker.getByLabel("Month").selectOption(period.month);
      await tracker.getByLabel("Year").selectOption(String(period.year));
      await expect(row).toContainText("Approved");

      await tracker.getByLabel("Submission status").selectOption({ label: "No Report" });
      await expect(row).toHaveCount(0);
    } finally {
      await send(request, "DELETE", `/reports/${id}`);
    }
  });
});

test.describe("as a plain user", () => {
  test.use({ storageState: authFile("user") });

  test("the API refuses the tracker", async ({ request }) => {
    const { month, year } = earlierPeriod(0);
    expect((await request.get(`${API}/admin/tracker?month=${month}&year=${year}`)).status()).toBe(403);
  });
});
