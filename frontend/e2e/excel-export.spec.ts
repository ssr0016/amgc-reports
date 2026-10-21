import { readFile } from "node:fs/promises";
import AxeBuilder from "@axe-core/playwright";
import { authFile, earlierPeriod, expect, send, test } from "./support";

test.use({ storageState: authFile("admin") });

test("an admin downloads the month and one report as Excel", async ({ page, request }) => {
  // A month no other spec lists, so this report never shows up in their empty-list checks.
  const period = earlierPeriod(5);
  const filed = await send(request, "POST", "/reports", {
    ...period, worker: "E2E Export Worker", church_name: "E2E Export Church", metrics: { worship_service: { week1: 12 } },
  });
  expect(filed.status(), await filed.text()).toBe(201);
  const { id } = await filed.json();
  try {
    await page.goto("/admin");
    const reports = page.getByRole("tabpanel", { name: "Reports" });
    await reports.getByLabel("Month").selectOption(period.month);
    await reports.getByLabel("Year").selectOption(String(period.year));
    const row = reports.getByRole("row").filter({ hasText: "E2E Export Worker" });
    await expect(row).toBeVisible();

    const [bulk] = await Promise.all([
      page.waitForEvent("download"),
      reports.getByRole("button", { name: "Download Bulk" }).click(),
    ]);
    expect(bulk.suggestedFilename()).toBe(`Church Growth Program Report ${period.month} ${period.year}.xlsx`);
    expect((await readFile((await bulk.path())!)).subarray(0, 2).toString()).toBe("PK"); // a zip, as .xlsx is
    await expect(reports.getByRole("button", { name: "Download Bulk" })).toBeEnabled();

    const [single] = await Promise.all([
      page.waitForEvent("download"),
      row.getByRole("button", { name: /^Excel / }).click(),
    ]);
    expect(single.suggestedFilename()).toBe(`E2E Export Church ${period.month} ${period.year}.xlsx`);

    expect((await new AxeBuilder({ page }).include('[role="tabpanel"]').analyze()).violations).toEqual([]);
  } finally {
    // Soft delete, so a later run can file the month again.
    expect((await send(request, "DELETE", `/reports/${id}`)).status()).toBe(204);
  }
});
