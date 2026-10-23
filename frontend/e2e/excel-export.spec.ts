import { readFile } from "node:fs/promises";
import AxeBuilder from "@axe-core/playwright";
import { authFile, earlierPeriod, expect, send, test } from "./support";

test.use({ storageState: authFile("admin") });

test("an admin previews, then downloads, the month and one report as Excel", async ({ page, request }) => {
  // A month no other spec lists, so this report never shows up in their empty-list checks.
  const period = earlierPeriod(5);
  const filed = await send(request, "POST", "/reports", {
    ...period, worker: "E2E Export Worker", church_name: "E2E Export Church", metrics: { worship_service: { week1: 12 } },
  });
  expect(filed.status(), await filed.text()).toBe(201);
  const { id } = await filed.json();
  try {
    // Only approved reports get a sheet in the bulk file (D-A16).
    expect((await send(request, "PATCH", `/reports/${id}/complete`)).status()).toBe(200);

    await page.goto("/admin");
    const reports = page.getByRole("tabpanel", { name: "Reports" });
    await reports.getByLabel("Month").selectOption(period.month);
    await reports.getByLabel("Year").selectOption(String(period.year));
    const row = reports.getByRole("row").filter({ hasText: "E2E Export Worker" });
    await expect(row).toBeVisible();

    // Each button opens a preview first (D-A15); only the preview's Download downloads.
    let downloads = 0;
    page.on("download", () => downloads++);

    await reports.getByRole("button", { name: "Download Bulk" }).click();
    let dialog = page.getByRole("alertdialog", { name: `Excel preview: ${period.month} ${period.year}` });
    const sheet = dialog.getByLabel("Sheet", { exact: true });
    await expect(sheet).toHaveValue("0");
    // The dashboard lists the e2e user's church; it filed nothing for this month.
    await expect(dialog.getByRole("table", { name: "Sheet Report Dashboard" }).getByRole("row").filter({ hasText: "Visionaires Christian Church" })).toContainText("No Report Submitted");
    await sheet.selectOption({ label: "E2E Export Church" });
    await expect(dialog.getByRole("table", { name: "Sheet E2E Export Church" })).toContainText("E2E Export Worker");
    expect((await new AxeBuilder({ page }).include('[role="alertdialog"]').analyze()).violations).toEqual([]);
    expect(downloads).toBe(0);
    const [bulk] = await Promise.all([page.waitForEvent("download"), dialog.getByRole("button", { name: "Download" }).click()]);
    expect(bulk.suggestedFilename()).toBe(`Church Growth Program Report ${period.month} ${period.year}.xlsx`);
    expect((await readFile((await bulk.path())!)).subarray(0, 2).toString()).toBe("PK"); // a zip, as .xlsx is
    await expect(dialog).toBeHidden();
    await expect(reports.getByRole("button", { name: "Download Bulk" })).toBeEnabled();

    await row.getByRole("button", { name: /^Excel / }).click();
    dialog = page.getByRole("alertdialog", { name: "Excel preview: E2E Export Church" });
    await expect(dialog.getByRole("table")).toContainText("E2E Export Worker");
    await dialog.getByRole("button", { name: "Cancel" }).click();
    await expect(dialog).toBeHidden();
    expect(downloads).toBe(1); // only the bulk file

    await row.getByRole("button", { name: /^Excel / }).click();
    const [single] = await Promise.all([page.waitForEvent("download"), dialog.getByRole("button", { name: "Download" }).click()]);
    expect(single.suggestedFilename()).toBe(`E2E Export Church ${period.month} ${period.year}.xlsx`);
    await expect(dialog).toBeHidden();

    expect((await new AxeBuilder({ page }).include('[role="tabpanel"]').analyze()).violations).toEqual([]);
  } finally {
    // Soft delete, so a later run can file the month again.
    expect((await send(request, "DELETE", `/reports/${id}`)).status()).toBe(204);
  }
});
