import { ACCOUNTS, API, authFile, clientIp, earlierPeriod, expect, previousPeriod, send, test } from "./support";

test.describe("as admin", () => {
  test.use({ storageState: authFile("admin") });

  test("an admin finds their own sign-in, a failed sign-in and an approval in the Logs section", async ({ page, request, playwright }) => {
    // A wrong password for the e2e user, then the right one, so the failed attempt does not count toward its lockout.
    const anon = await playwright.request.newContext({ extraHTTPHeaders: { "CF-Connecting-IP": clientIp("audit-logs-failed") } });
    const csrf = async () => (await (await anon.get(`${API}/auth/csrf`)).json()).csrf_token;
    const login = async (password: string) =>
      anon.post(`${API}/auth/login`, { data: { email: ACCOUNTS.user.email, password }, headers: { "X-CSRF-Token": await csrf() } });
    expect((await login("not-the-password")).status()).toBe(401);
    expect((await login(ACCOUNTS.user.password)).status()).toBe(200);

    // A report approved now, so its APPROVE_REPORT row falls in the current month.
    const period = earlierPeriod(7);
    const filed = await send(anon, "POST", "/reports", { ...previousPeriod(), worker: "E2E Audit Worker" });
    expect(filed.status(), await filed.text()).toBe(201);
    const { id } = await filed.json();
    await anon.dispose();
    try {
      const moved = await send(request, "PUT", `/reports/${id}`, { ...period, worker: "E2E Audit Worker" });
      expect(moved.status(), await moved.text()).toBe(200);
      expect((await send(request, "PATCH", `/reports/${id}/complete`)).status()).toBe(200);

      await page.goto("/admin?tab=logs");
      const logs = page.getByRole("region", { name: "Logs" });
      await expect(logs.getByText(/total logs$/)).toBeVisible();

      await logs.getByLabel("Action").selectOption({ label: "Logged in" });
      await expect(logs.getByRole("row").filter({ hasText: ACCOUNTS.admin.name }).first()).toBeVisible();

      await logs.getByLabel("Action").selectOption({ label: "Failed login" });
      const failed = logs.getByRole("row").filter({ hasText: ACCOUNTS.user.email }).first();
      await expect(failed).toBeVisible();
      await failed.getByRole("button", { name: /View details/ }).click();
      await expect(logs.getByText("Wrong password").first()).toBeVisible();

      await logs.getByLabel("Action").selectOption({ label: "Approved report" });
      const approved = logs.getByRole("row").filter({ hasText: `${period.month} ${period.year}` }).first();
      await expect(approved).toContainText(ACCOUNTS.admin.name);
      const button = approved.getByRole("button", { name: /View details/ });
      await button.click();
      await expect(button).toHaveAttribute("aria-expanded", "true");
      await expect(page.locator(`#${await button.getAttribute("aria-controls")}`)).toContainText("Period");
      await expect(logs).not.toContainText(/\b\d{1,3}(\.\d{1,3}){3}\b/); // no IP address, even in the details (D-A4)
    } finally {
      await send(request, "DELETE", `/reports/${id}`);
    }
  });
});

test.describe("as a plain user", () => {
  test.use({ storageState: authFile("user") });

  test("has no Logs link, and the API refuses the audit trail", async ({ page, request }) => {
    await page.goto("/admin");
    await expect(page.getByText("You are not allowed to view the admin panel.")).toBeVisible();
    await expect(page.getByRole("link", { name: "Admin Logs", exact: true })).toHaveCount(0);
    const { month, year } = earlierPeriod(0);
    expect((await request.get(`${API}/admin/audit-logs?month=${month}&year=${year}`)).status()).toBe(403);
  });
});
