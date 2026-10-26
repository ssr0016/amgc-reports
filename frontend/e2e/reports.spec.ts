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
    await page.getByRole("button", { name: "Submit Report" }).click();

    await expect(page).toHaveURL(/\/reports$/);
    await expect(page.getByRole("heading", { name: "My Reports", exact: true })).toBeVisible();
    await expect(page.getByRole("link", { name: "New Report", exact: true })).toBeVisible();
    await expect(page.getByRole("region", { name: "Report statistics" })).toHaveCount(0);
    const row = rowOf(page, ACCOUNTS.user.name);
    await expect(row).toContainText(period.month);
    await expect(row).toContainText("Pending");
  });

  test("a second report for the same month is refused and keeps the form", async ({ page }) => {
    await page.goto("/reports/new");
    await page.getByLabel("Narrative Report").fill("Do not lose me.");
    await page.getByRole("button", { name: "Submit Report" }).click();
    await expect(page.getByText("You already have a report for this month and year.")).toBeVisible();
    await expect(page).toHaveURL(/\/reports\/new$/);
    await expect(page.getByLabel("Narrative Report")).toHaveValue("Do not lose me.");
  });

  test("a bad metric is caught before anything is sent", async ({ page }) => {
    await page.goto("/reports/new");
    await page.getByLabel("Outreach, week 1").fill("1.5");
    await page.getByLabel("Outreach, week 1").blur();
    await expect(page.getByText("Must be a whole number", { exact: true })).toBeVisible();
    await page.getByRole("button", { name: "Submit Report" }).click();
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
    await page.getByRole("button", { name: "Save Changes" }).click();
    await expect(page).toHaveURL(/\/reports$/);
    await expect(rowOf(page, "E2E User Edited")).toBeVisible();
  });

  test("cannot approve: no button, and the API says 403", async ({ page, request }) => {
    await page.goto("/reports");
    await expect(rowOf(page, "E2E User Edited")).toBeVisible();
    await expect(page.getByRole("button", { name: /^(Approve|Unapprove) / })).toHaveCount(0);

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
    await expect(page.getByText("No reports match these filters.")).toBeVisible();
    const id = (await (await request.get("http://localhost:8081/api/v1/reports?limit=1")).json()).data[0].id;
    await page.goto(`/reports/${id}/edit`);
    await expect(page.getByLabel("Worker Name")).toHaveValue("E2E User Edited"); // write_any
    await expect(page.getByRole("button", { name: "Save Changes" })).toBeVisible();
    await expect(page.getByRole("button", { name: /^(Approve|Unapprove) / })).toHaveCount(0);
    expect((await send(request, "PATCH", `/reports/${id}/complete`)).status()).toBe(403);
  });

  test("cannot create reports through the UI or API", async ({ page, request }) => {
    await page.goto("/reports");
    await expect(page.getByRole("link", { name: "New Report" })).toHaveCount(0);
    await page.goto("/reports/new");
    await expect(page.getByText("You are not allowed to create reports.")).toBeVisible();
    await expect(page.getByRole("button", { name: "Submit Report" })).toHaveCount(0);
    for (const p of [period, earlierPeriod(1)]) {
      const res = await send(request, "POST", "/reports", { ...p, worker: "Forbidden Editor" });
      expect(res.status(), await res.text()).toBe(403);
    }
  });
});

test.describe("admin", () => {
  test.use({ storageState: authFile("admin") });

  // Approval refreshes the shared list and its page-level counts.
  test("approves and unapproves from the admin panel, and the stats cards follow", async ({ page }) => {
    await page.goto("/admin");
    await expect(page.getByRole("heading", { name: "All Reports", exact: true })).toBeVisible();
    const card = (name: string) =>
      page
        .getByRole("region", { name: "Report statistics" })
        .locator("[data-slot=card]")
        .filter({ has: page.getByText(name, { exact: true }) })
        .locator(".font-mono");
    const row = () => rowOf(page, "E2E User Edited");

    await expect(page.getByText(/\d+ total reports/)).toBeVisible();
    await expect(page.getByRole("columnheader")).toHaveText([
      "", "Worker", "Month", "Year", "Area", "Church", "Status", "Actions",
    ]);
    await expect(page.locator("main header").getByRole("button", { name: "Download Bulk" })).toBeVisible();
    await expect(row().getByText("Pending")).toBeVisible(); // the tab opens on the previous month
    await expect(card("Approved")).toBeVisible();
    const approved = Number(await card("Approved").innerText());
    const pending = Number(await card("Pending").innerText());

    await row().getByRole("button", { name: /^Approve / }).click();
    await expect(row().getByText("Approved")).toBeVisible();
    await expect(page.getByText(`Report approved: ${label}`)).toBeVisible();
    await expect(row().getByRole("button", { name: /^Unapprove / })).toBeVisible();
    await expect(card("Approved")).toHaveText(String(approved + 1));
    await expect(card("Pending")).toHaveText(String(pending - 1));

    await page.reload(); // the server kept it
    await expect(row().getByText("Approved")).toBeVisible();
    await row().getByRole("button", { name: /^Unapprove / }).click();
    await expect(row().getByText("Pending")).toBeVisible();
    await expect(card("Approved")).toHaveText(String(approved));
    await expect(card("Pending")).toHaveText(String(pending));

    // The next tests expect the report Pending, so wait for the server to say so.
    await page.reload();
    await expect(row().getByText("Pending")).toBeVisible();
  });

  test("the personal list excludes other users' reports even for an admin", async ({ page, request }) => {
    await page.goto("/reports");
    await expect(page.getByRole("heading", { name: "My Reports", exact: true })).toBeVisible();
    await expect(page.getByText("No reports match these filters.")).toBeVisible();
    await expect(rowOf(page, "E2E User Edited")).toHaveCount(0);
    await expect(page.getByRole("region", { name: "Report statistics" })).toHaveCount(0);
    const personal = await request.get("http://localhost:8081/api/v1/reports?mine=true");
    expect(personal.status()).toBe(200);
    expect((await personal.json()).meta.total).toBe(0);
  });

  test("cannot create reports through the UI or API", async ({ page, request }) => {
    await page.goto("/reports");
    await expect(page.getByRole("link", { name: "New Report" })).toHaveCount(0);
    await page.goto("/reports/new");
    await expect(page.getByText("You are not allowed to create reports.")).toBeVisible();
    await expect(page.getByRole("button", { name: "Submit Report" })).toHaveCount(0);
    for (const p of [period, earlierPeriod(2)]) {
      const res = await send(request, "POST", "/reports", { ...p, worker: "Forbidden Admin" });
      expect(res.status(), await res.text()).toBe(403);
    }
  });

  test("lists another user's reports", async ({ page }) => {
    await page.goto("/admin?tab=reports");
    await expect(rowOf(page, "E2E User Edited")).toBeVisible();
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
    await page.getByRole("button", { name: "Submit Report" }).click();
    await expect(page).toHaveURL(/\/reports$/);
    await expect(rowOf(page, ACCOUNTS.user.name)).toBeVisible();
  });
});
