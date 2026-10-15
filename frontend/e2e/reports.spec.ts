import type { Page } from "@playwright/test";
import { ACCOUNTS, authFile, earlierPeriod, expect, previousPeriod, send, test } from "./support";

const period = previousPeriod();
const label = `${period.month} ${period.year}`;
const rowOf = (page: Page, text: string) => page.getByRole("row", { name: new RegExp(text) });

// The specs share one database and run in order: the user files a report, others act on it.
test.describe.configure({ mode: "serial" });

test.describe("user", () => {
  test.use({ storageState: authFile("user") });

  test("the new report form is filled from the profile and locked to the previous month", async ({ page }) => {
    await page.goto("/reports/new");
    await expect(page.getByLabel("Worker Name")).toHaveValue(ACCOUNTS.user.name);
    await expect(page.getByLabel("Area Assignment")).toHaveValue(ACCOUNTS.user.area_assignment);
    await expect(page.getByLabel("Church Name")).toHaveValue(ACCOUNTS.user.church_name);
    await expect(page.getByLabel("Month")).toHaveValue(period.month);
    await expect(page.getByLabel("Month")).toBeDisabled();
    await expect(page.getByLabel("Year")).toHaveValue(String(period.year));
  });

  test("creates a report and finds it in the list", async ({ page }) => {
    await page.goto("/reports/new");
    await page.getByLabel("Outreach, week 1").fill("3");
    await page.getByLabel("Tithes & Offering, week 2").fill("1500.50");
    await page.getByLabel("Narrative Report").fill("A quiet, good month.");
    await page.getByRole("button", { name: "Create Report" }).click();

    await expect(page).toHaveURL(/\/reports$/);
    const row = rowOf(page, ACCOUNTS.user.name);
    await expect(row).toContainText(period.month);
    await expect(row).toContainText("Pending");
  });

  test("a second report for the same month is refused and keeps the form", async ({ page }) => {
    await page.goto("/reports/new");
    await page.getByLabel("Narrative Report").fill("Do not lose me.");
    await page.getByRole("button", { name: "Create Report" }).click();
    await expect(page.getByText("You already have a report for this month and year.")).toBeVisible();
    await expect(page).toHaveURL(/\/reports\/new$/);
    await expect(page.getByLabel("Narrative Report")).toHaveValue("Do not lose me.");
  });

  test("a bad metric is caught before anything is sent", async ({ page }) => {
    await page.goto("/reports/new");
    await page.getByLabel("Outreach, week 1").fill("1.5");
    await page.getByLabel("Outreach, week 1").blur();
    await expect(page.getByText("Must be a whole number", { exact: true })).toBeVisible();
    await page.getByRole("button", { name: "Create Report" }).click();
    await expect(page.getByText("Fix the highlighted fields and try again.")).toBeVisible();
    await expect(page).toHaveURL(/\/reports\/new$/);
  });

  test("edits the report: the period stays locked and the saved values come back", async ({ page }) => {
    await page.goto("/reports");
    await rowOf(page, ACCOUNTS.user.name).getByRole("link", { name: "Edit" }).click();
    await expect(page.getByRole("heading", { name: "Edit report" })).toBeVisible();
    await expect(page.getByLabel("Outreach, week 1")).toHaveValue("3");
    await expect(page.getByLabel("Tithes & Offering, week 2")).toHaveValue("1500.5");
    await expect(page.getByLabel("Month")).toBeDisabled();

    await page.getByLabel("Worker Name").fill("E2E User Edited");
    await page.getByLabel("Outreach, week 1").fill("4");
    await page.getByRole("button", { name: "Update Report" }).click();
    await expect(page).toHaveURL(/\/reports$/);
    await expect(rowOf(page, "E2E User Edited")).toBeVisible();
  });

  test("cannot approve: no button, and the API says 403", async ({ page, request }) => {
    await page.goto("/reports");
    await expect(rowOf(page, "E2E User Edited")).toBeVisible();
    await expect(page.getByRole("button", { name: /approve/i })).toHaveCount(0);

    const id = (await (await request.get("http://localhost:8081/api/v1/reports?limit=1")).json()).data[0].id;
    expect((await send(request, "PATCH", `/reports/${id}/complete`)).status()).toBe(403);
  });

  test("late reports are refused by the API for a user (422 on month)", async ({ request }) => {
    for (const p of [earlierPeriod(1), { month: period.month, year: period.year - 1 }]) {
      const res = await send(request, "POST", "/reports", { month: p.month, year: p.year });
      expect(res.status(), `${p.month} ${p.year}`).toBe(422);
      expect((await res.json()).details.month).toBe("Reports can only be created for the previous month.");
    }
  });
});

test.describe("editor", () => {
  test.use({ storageState: authFile("editor") });

  test("can open and edit someone else's report, but not approve it", async ({ page, request }) => {
    await page.goto("/reports");
    const row = rowOf(page, "E2E User Edited");
    await expect(row.getByRole("link", { name: "Edit" })).toBeVisible(); // write_any
    await expect(page.getByRole("button", { name: /approve/i })).toHaveCount(0);

    const id = (await (await request.get("http://localhost:8081/api/v1/reports?limit=1")).json()).data[0].id;
    expect((await send(request, "PATCH", `/reports/${id}/complete`)).status()).toBe(403);
  });

  test("may file a late report", async ({ request }) => {
    const p = earlierPeriod(1);
    const res = await send(request, "POST", "/reports", { month: p.month, year: p.year, worker: "Late Editor" });
    expect(res.status()).toBe(201);
  });
});

test.describe("admin", () => {
  test.use({ storageState: authFile("admin") });

  test("approves and unapproves a report in the list", async ({ page }) => {
    await page.goto("/reports");
    const row = rowOf(page, "E2E User Edited");
    await row.getByRole("button", { name: "Approve" }).click();
    await expect(row.getByText("Completed")).toBeVisible();
    await expect(page.getByText(`Approved ${label}`)).toBeVisible();

    await page.reload(); // the server kept it
    await expect(rowOf(page, "E2E User Edited").getByText("Completed")).toBeVisible();
    await rowOf(page, "E2E User Edited").getByRole("button", { name: "Unapprove" }).click();
    await expect(rowOf(page, "E2E User Edited").getByText("Pending")).toBeVisible();
  });

  test("may file a late report", async ({ request }) => {
    const p = earlierPeriod(2);
    const res = await send(request, "POST", "/reports", { month: p.month, year: p.year, worker: "Late Admin" });
    expect(res.status()).toBe(201);
  });

  test("lists every user's reports", async ({ page }) => {
    await page.goto(`/reports?month=&year=${period.year}`);
    await expect(rowOf(page, "E2E User Edited")).toBeVisible();
    await expect(rowOf(page, "Late Editor")).toBeVisible();
  });
});

test.describe("user, later", () => {
  test.use({ storageState: authFile("user") });

  test("deletes the report after confirming, and can file the month again", async ({ page }) => {
    await page.goto("/reports");
    await rowOf(page, "E2E User Edited").getByRole("button", { name: "Delete" }).click();
    await expect(page.getByRole("alertdialog")).toContainText(`Delete ${label}?`);
    await page.getByRole("alertdialog").getByRole("button", { name: "Delete" }).click();
    await expect(page.getByText("No reports match these filters.")).toBeVisible();

    await page.goto("/reports/new");
    await page.getByRole("button", { name: "Create Report" }).click();
    await expect(page).toHaveURL(/\/reports$/);
    await expect(rowOf(page, ACCOUNTS.user.name)).toBeVisible();
  });
});
