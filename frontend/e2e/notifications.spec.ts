import { ACCOUNTS, authFile, expect, send, test } from "./support";

test.describe("in-app notifications", () => {
  test.use({ storageState: authFile("admin") });

  test("shows a submitted report and marks it read", async ({ page, playwright, clientIp }) => {
    const worker = await playwright.request.newContext({
      storageState: authFile("notifier"),
      extraHTTPHeaders: { "CF-Connecting-IP": clientIp },
    });
    const report = await send(worker, "POST", "/reports", {
      month: "October",
      year: 2026,
      metrics: { outreach: { week1: 2 } },
    });
    expect(report.status(), await report.text()).toBe(201);
    await worker.dispose();

    await page.goto("/reports");
    const bell = page.getByRole("button", { name: /Notifications, 1 unread/ });
    await expect(bell).toBeVisible();
    await bell.click();
    await expect(page.getByRole("menuitem", { name: `Report submitted by ${ACCOUNTS.notifier.name}` })).toBeVisible();
    await page.getByRole("menuitem", { name: `Report submitted by ${ACCOUNTS.notifier.name}` }).click();
    await expect(page).toHaveURL(/\/admin\?tab=reports$/);
    await expect(page.getByRole("button", { name: /Notifications, no unread/ })).toBeVisible();
  });
});
