import AxeBuilder from "@axe-core/playwright";
import { API, authFile, expect, previousPeriod, send, test } from "./support";

test.use({ storageState: authFile("admin") });

test("reports retain deleted and deceased owners, update badges, and preserve reader privacy", async ({ page, request, playwright, clientIp }) => {
  const account = { email: "e2e-report-owner@example.com", name: "E2E Historical Owner", password: "E2eOwner!23456" };
  const roles: { id: number; name: string }[] = (await (await request.get(`${API}/admin/roles?limit=100`)).json()).data;
  const created = await send(request, "POST", "/admin/users", {
    ...account, confirm_password: account.password, role_id: roles.find((r) => r.name === "user")!.id,
  });
  expect(created.status(), await created.text()).toBe(201);
  const owner = await created.json();
  const ownerRequest = await playwright.request.newContext({ extraHTTPHeaders: { "X-Real-IP": clientIp } });
  let reportId: number;
  try {
    expect((await send(ownerRequest, "POST", "/auth/login", account)).status()).toBe(200);
    const res = await send(ownerRequest, "POST", "/reports", {
      ...previousPeriod(), worker: account.name, narrative_report: "Private pastoral history",
      metrics: { outreach: { week1: 3 }, tithes_offering: { week1: 100 } },
    });
    expect(res.status(), await res.text()).toBe(201);
    const report = await res.json();
    reportId = report.id;
    expect(report.owner_status).toBe("active");
    expect(report.owner_deleted_at).toBeUndefined();
  } finally {
    await ownerRequest.dispose();
  }

  await page.goto("/admin");
  const reports = page.getByRole("region", { name: "Reports" });
  const reportRow = reports.getByRole("row").filter({ hasText: account.name });
  await expect(reportRow.getByText("Pending", { exact: true })).toBeVisible();
  await expect(reportRow.getByText("Archived", { exact: true })).toHaveCount(0);

  await page.goto(`/admin/users/${owner.id}/edit`);
  const status = page.getByRole("group", { name: "Status" });
  await status.getByLabel("Status").selectOption("deceased");
  await status.getByRole("button", { name: "Update status" }).click();
  await page.getByRole("alertdialog").getByRole("button", { name: "Change status" }).click();
  await expect(page.getByText(`${account.name} is now Deceased`)).toBeVisible();
  await page.getByRole("link", { name: "Admin Reports", exact: true }).click();
  await expect(reportRow.getByText("Archived", { exact: true })).toBeVisible();

  await page.getByRole("link", { name: "Admin Users", exact: true }).click();
  const users = page.getByRole("region", { name: "Users" });
  await users.getByRole("button", { name: `Delete ${account.name}`, exact: true }).click();
  await page.getByRole("alertdialog").getByRole("button", { name: "Delete user", exact: true }).click();
  await expect(page.getByRole("alertdialog")).toHaveCount(0);
  await page.getByRole("link", { name: "Admin Reports", exact: true }).click();
  await expect(reportRow.getByText("Deleted user", { exact: true })).toBeVisible();
  await expect(reportRow.getByText("Archived", { exact: true })).toBeVisible();
  await expect(reportRow.getByRole("button", { name: /^Approve / })).toBeEnabled();
  expect((await new AxeBuilder({ page }).include('main section[aria-label="Reports"]').analyze()).violations).toEqual([]);

  const reader = await playwright.request.newContext({ storageState: authFile("user"), extraHTTPHeaders: { "X-Real-IP": clientIp } });
  try {
    const res = await reader.get(`${API}/reports/${reportId}`);
    expect(res.status()).toBe(200);
    const detail = await res.json();
    expect(detail.owner_status).toBe("deceased");
    expect(detail.owner_deleted_at).toBeTruthy();
    expect(detail.narrative_report).toBeUndefined();
    expect(detail.metrics.tithes_offering).toBeUndefined();
    expect(detail.metrics.outreach.week1).toBe(3);
  } finally {
    await reader.dispose();
  }
  await page.goto("/reports");
  const regularRow = page.getByRole("row").filter({ hasText: account.name });
  await expect(regularRow.getByText("Deleted user", { exact: true })).toBeVisible();
  await expect(regularRow.getByText("Archived", { exact: true })).toBeVisible();

  await page.goto("/admin");
  await page.getByRole("link", { name: "Admin Users", exact: true }).click();
  await users.getByLabel("Status").selectOption("deleted");
  await users.getByRole("button", { name: `Restore ${account.name}`, exact: true }).click();
  await expect(page.getByText(`${account.name} restored`, { exact: true })).toBeVisible();
  await page.getByRole("link", { name: "Admin Reports", exact: true }).click();
  await expect(reportRow.getByText("Deleted user", { exact: true })).toHaveCount(0);
  await expect(reportRow.getByText("Archived", { exact: true })).toBeVisible();

  await page.goto(`/admin/users/${owner.id}/edit`);
  await status.getByLabel("Status").selectOption("active");
  await status.getByRole("button", { name: "Update status" }).click();
  await expect(page.getByText(`${account.name} is now Active`)).toBeVisible();
  await page.getByRole("link", { name: "Admin Reports", exact: true }).click();
  await expect(reportRow.getByText("Archived", { exact: true })).toHaveCount(0);
  await expect(reportRow.getByText("Pending", { exact: true })).toBeVisible();
  // Leave shared report totals unchanged for the other browser specs.
  expect((await send(request, "DELETE", `/reports/${reportId}`)).status()).toBe(204);
});
