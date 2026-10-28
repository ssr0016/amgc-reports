import { ACCOUNTS, API, authFile, expect, previousPeriod, send, test } from "./support";

test.use({ storageState: authFile("admin") });

test("analytics leads with every church and metric, and the metric picker drives only the trend", async ({ page, request, playwright, clientIp }) => {
  // A plain user may file only the previous month; another spec may have left a report there, so start clean.
  const period = previousPeriod();
  const author = await playwright.request.newContext({ storageState: authFile("user"), extraHTTPHeaders: { "CF-Connecting-IP": clientIp } });
  const old = await (await author.get(`${API}/reports?month=${period.month}&year=${period.year}`)).json();
  for (const r of old.data) await send(author, "DELETE", `/reports/${r.id}`);
  const filed = await send(author, "POST", "/reports", {
    ...period,
    metrics: { worship_service: { week1: 40, week2: 45 }, tithes_offering: { week1: 500 }, baptism: { week2: 2 }, person_evangelized: { week1: 3 } },
  });
  expect(filed.status(), await filed.text()).toBe(201);
  const { id } = await filed.json();
  await author.dispose();
  const silent = { email: "e2e-analytics-silent@example.com", password: "E2eSilent!23456", church: "E2E Silent Church" };
  const created = await send(request, "POST", "/admin/users", {
    email: silent.email, name: "E2E Silent", password: silent.password, confirm_password: silent.password, church_name: silent.church,
  });
  expect(created.status(), await created.text()).toBe(201);
  const { id: silentId } = await created.json();
  try {
    // The default status is Approved, so approve the report.
    expect((await send(request, "PATCH", `/reports/${id}/complete`)).status()).toBe(200);

    await page.goto("/admin?tab=analytics");
    await page.getByLabel("Year", { exact: true }).selectOption(String(period.year));
    const kpis = page.getByRole("region", { name: "Analytics summary" });
    for (const label of ["Avg weekly worship attendance", "Tithes & Offering", "Baptisms", "Persons evangelized", "Churches reporting"]) {
      await expect(kpis.getByText(label, { exact: true })).toBeVisible();
    }
    // 40 + 45 over the 2 weeks with reports, not over every week of the year.
    await expect(kpis).toContainText("42.5");
    await expect(kpis).toContainText("÷ 2 weeks with reports");
    await expect(page.getByLabel("Analytics scope")).toContainText(`Full Year ${period.year} · Approved reports ·`);

    // The matrix sits above the trend section.
    const matrixHeading = page.getByRole("heading", { name: "Full Metrics Summary" });
    const trendHeading = page.getByRole("heading", { name: "Trend", exact: true });
    expect((await matrixHeading.boundingBox())!.y).toBeLessThan((await trendHeading.boundingBox())!.y);
    await expect(page.getByLabel("Trend for")).toHaveValue("worship_service");

    // Every church is a column: the reporting one with its values, the silent one as No report.
    const matrix = page.getByRole("table", { name: "Full metrics summary" });
    const headers = matrix.getByRole("row").first();
    await expect(headers).toContainText(ACCOUNTS.user.church_name);
    await expect(headers).toContainText(silent.church);
    await expect(matrix.getByRole("row").filter({ hasText: "Worship Service" })).toContainText("85");
    await expect(matrix.getByRole("row").filter({ hasText: "Worship Service" })).toContainText("No report");
    await expect(matrix.getByText("No report", { exact: true })).toHaveCount(21);
    for (const group of ["Attendance", "Finance", "Pastoral Work", "Evangelism & Growth", "Ministry & Events"]) {
      await expect(matrix.getByRole("columnheader", { name: group, exact: true })).toBeVisible();
    }

    // Changing the trend metric leaves the KPIs and the matrix as they were.
    const kpiText = await kpis.innerText();
    const matrixText = await matrix.innerText();
    await page.getByLabel("Trend for").selectOption("tithes_offering");
    await expect(page.getByRole("table", { name: "Monthly trend data" })).toContainText("₱500.00");
    expect(await kpis.innerText()).toBe(kpiText);
    expect(await matrix.innerText()).toBe(matrixText);
    await page.getByLabel("Trend for").selectOption("worship_service");
    await expect(page.getByRole("table", { name: "Monthly trend data" })).toContainText("85");

    await page.screenshot({ path: "test-results/analytics.png", fullPage: true });
  } finally {
    expect((await send(request, "DELETE", `/reports/${id}`)).status()).toBe(204);
    expect((await send(request, "DELETE", `/admin/users/${silentId}`)).status()).toBe(200);
  }
});
