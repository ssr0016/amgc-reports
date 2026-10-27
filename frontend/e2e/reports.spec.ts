import type { Page } from "@playwright/test";
import { ACCOUNTS, API, authFile, earlierPeriod, expect, previousPeriod, send, test } from "./support";

const period = previousPeriod();
const label = `${period.month} ${period.year}`;
const rowOf = (page: Page, text: string) => page.getByRole("row", { name: new RegExp(text) });
// Worker, area and church are read-only profile details: a <dt> and its <dd>.
const profileValue = (page: Page, term: string) => page.getByRole("term").filter({ hasText: term }).locator("+ dd");

// The specs share one database and run in order: the user files a report, others act on it.
test.describe.configure({ mode: "serial" });

test.describe("user", () => {
  test.use({ storageState: authFile("user") });

  test("the new report form is filled from the profile and locked to the previous month", async ({ page }) => {
    await page.goto("/reports/new");
    await expect(profileValue(page, "Worker Name")).toHaveText(ACCOUNTS.user.name);
    await expect(profileValue(page, "Area Assignment")).toHaveText(ACCOUNTS.user.area_assignment);
    await expect(profileValue(page, "Church Name")).toHaveText(ACCOUNTS.user.church_name);
    await expect(page.getByLabel("Month")).toHaveValue(period.month);
    await expect(page.getByLabel("Month")).toBeDisabled();
    await expect(page.getByLabel("Year")).toHaveValue(String(period.year));
  });

  // Before "creates a report": once the month has a report, /reports/new redirects to it.
  test("a bad metric is caught before anything is sent", async ({ page }) => {
    await page.goto("/reports/new");
    await page.getByLabel("Outreach, week 1").fill("1.5");
    await page.getByLabel("Outreach, week 1").blur();
    await expect(page.getByText("Must be a whole number", { exact: true })).toBeVisible();
    await page.getByRole("button", { name: "Submit Report" }).click();
    await expect(page.getByText("Fix the highlighted fields and try again.")).toBeVisible();
    await expect(page).toHaveURL(/\/reports\/new$/);
  });

  test("creates a report and finds it in the list", async ({ page }) => {
    await page.goto("/reports/new");
    await page.getByLabel("Outreach, week 1").fill("3");
    await page.getByLabel("Tithes & Offering, week 2").fill("1500.50");
    await page.getByLabel("Narrative Report").fill("A quiet, good month.");
    await page.getByRole("button", { name: "Submit Report" }).click();

    await expect(page).toHaveURL(/\/reports$/);
    await expect(page.getByRole("heading", { name: "Reports", exact: true })).toBeVisible();
    await expect(page.getByRole("link", { name: "New Report", exact: true })).toBeVisible();
    await expect(page.getByRole("region", { name: "Report statistics" })).toHaveCount(0);
    const row = rowOf(page, ACCOUNTS.user.name);
    await expect(row).toContainText(period.month);
    await expect(row).toContainText("Pending");
  });

  test("personal search updates automatically and Clear restores the list", async ({ page }) => {
    await page.goto("/reports");
    await expect(rowOf(page, ACCOUNTS.user.name)).toBeVisible();
    await expect(page.getByRole("button", { name: "Search", exact: true })).toHaveCount(0);
    await expect(page.getByRole("button", { name: "Download Bulk" })).toHaveCount(0);
    await expect(page.getByRole("button", { name: /Notifications/ })).toHaveCount(0);
    await expect(page.getByRole("button", { name: "Clear", exact: true })).toHaveCount(0);
    const month = await page.getByLabel("Month", { exact: true }).inputValue();
    const year = await page.getByLabel("Year", { exact: true }).inputValue();
    await page.getByLabel("Church", { exact: true }).fill(ACCOUNTS.user.church_name);
    await page.getByLabel("Area", { exact: true }).fill(ACCOUNTS.user.area_assignment);
    await page.getByLabel("Worker", { exact: true }).fill("No such worker");
    await expect(page.getByText("No reports found", { exact: true })).toBeVisible();
    await page.getByRole("button", { name: "Clear", exact: true }).click();
    for (const label of ["Worker", "Church", "Area"]) await expect(page.getByLabel(label, { exact: true })).toHaveValue("");
    await expect(page.getByLabel("Month", { exact: true })).toHaveValue(month);
    await expect(page.getByLabel("Year", { exact: true })).toHaveValue(year);
    await expect(rowOf(page, ACCOUNTS.user.name)).toBeVisible();
    await expect(page.getByRole("button", { name: "Clear", exact: true })).toHaveCount(0);
    await page.screenshot({ path: "/tmp/amg-personal-reports-final.png", fullPage: true });
  });

  // Since c9ed71e/065660a the form is not shown for a month that already has a report.
  test("a second report for the same month opens the existing one, and the API refuses it", async ({ page, request }) => {
    await page.goto("/reports/new");
    await expect(page).toHaveURL(/\/reports\/\d+$/); // submitted (Pending), so the read-only view
    await expect(page.getByRole("button", { name: "Submit Report" })).toHaveCount(0);
    const res = await send(request, "POST", "/reports", period);
    expect(res.status(), await res.text()).toBe(409);
  });

  test("edits the report: the period stays locked and the saved values come back", async ({ page }) => {
    await page.goto("/reports");
    await rowOf(page, ACCOUNTS.user.name).getByRole("link", { name: "Edit" }).click();
    await expect(page.getByRole("heading", { name: "Edit report" })).toBeVisible();
    await expect(page.getByLabel("Outreach, week 1")).toHaveValue("3");
    await expect(page.getByLabel("Tithes & Offering, week 2")).toHaveValue("1500.5");
    await expect(page.getByLabel("Month")).toBeDisabled();

    await page.getByLabel("Outreach, week 1").fill("4");
    await page.getByRole("button", { name: "Submit Report" }).click();
    await expect(page).toHaveURL(/\/reports$/);
    await expect(rowOf(page, ACCOUNTS.user.name)).toBeVisible();
  });

  test("cannot approve: no button, and the API says 403", async ({ page, request }) => {
    await page.goto("/reports");
    await expect(rowOf(page, ACCOUNTS.user.name)).toBeVisible();
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
    await expect(page.getByRole("row").first()).toBeVisible();
    const id = (await (await request.get("http://localhost:8081/api/v1/reports?limit=1")).json()).data[0].id;
    await page.goto(`/reports/${id}/edit`);
    await expect(profileValue(page, "Worker Name")).toHaveText(ACCOUNTS.user.name); // write_any
    await expect(page.getByRole("button", { name: "Submit Report" })).toBeVisible();
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

  test("management search updates automatically and Clear restores the list", async ({ page }) => {
    await page.goto("/admin?tab=reports");
    await expect(rowOf(page, ACCOUNTS.user.name)).toBeVisible();
    await expect(page.getByRole("button", { name: "Search", exact: true })).toHaveCount(0);
    await expect(page.getByRole("button", { name: /Notifications/ })).toBeVisible();
    await expect(page.getByRole("button", { name: "Download Bulk" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Clear", exact: true })).toHaveCount(0);
    const month = await page.getByLabel("Month", { exact: true }).inputValue();
    const year = await page.getByLabel("Year", { exact: true }).inputValue();
    await page.getByLabel("Worker", { exact: true }).fill(ACCOUNTS.user.name);
    await page.getByLabel("Area", { exact: true }).fill(ACCOUNTS.user.area_assignment);
    await page.getByLabel("Church", { exact: true }).fill("No such church");
    await expect(page.getByText("No reports match these filters", { exact: true })).toBeVisible();
    await page.getByRole("button", { name: "Clear", exact: true }).click();
    for (const label of ["Worker", "Church", "Area"]) await expect(page.getByLabel(label, { exact: true })).toHaveValue("");
    await expect(page.getByLabel("Month", { exact: true })).toHaveValue(month);
    await expect(page.getByLabel("Year", { exact: true })).toHaveValue(year);
    await expect(rowOf(page, ACCOUNTS.user.name)).toBeVisible();
    await expect(page.getByRole("button", { name: "Clear", exact: true })).toHaveCount(0);
    await page.screenshot({ path: "/tmp/amg-admin-reports-final.png", fullPage: true });
  });

  // Approval refreshes the shared list and its page-level counts.
  test("approves and sends a report back for revision, and the stats cards follow", async ({ page }) => {
    await page.goto("/admin");
    await expect(page.getByRole("heading", { name: "All reports", exact: true })).toBeVisible();
    const card = (name: string) =>
      page
        .getByRole("region", { name: "Report statistics" })
        .locator("[data-slot=card]")
        .filter({ has: page.getByText(name, { exact: true }) })
        .locator("[data-slot=card-title]");
    const row = () => rowOf(page, ACCOUNTS.user.name);

    await expect(page.getByText(/\d+ total reports/)).toBeVisible();
    await expect(page.getByRole("columnheader")).toHaveText([
      "", "Worker", "Month", "Year", "Area", "Church", "Status", "Actions",
    ]);
    await expect(page.locator("main header").getByRole("button", { name: "Download Bulk" })).toBeVisible();
    await expect(row().getByText("Pending")).toBeVisible(); // the tab opens on the previous month
    await expect(card("Approved")).toBeVisible();
    const approved = Number(await card("Approved").innerText());
    const pending = Number(await card("Pending").innerText());
    const needsRevision = Number(await card("Needs Revision").innerText());

    await row().getByRole("button", { name: /^Approve / }).click();
    await expect(row().getByText("Approved")).toBeVisible();
    await expect(page.getByText(`Report approved: ${label}`)).toBeVisible();
    await expect(row().getByRole("button", { name: /^Unapprove / })).toBeVisible();
    await expect(card("Approved")).toHaveText(String(approved + 1));
    await expect(card("Pending")).toHaveText(String(pending - 1));

    await page.reload(); // the server kept it
    await expect(row().getByText("Approved")).toBeVisible();
    await row().getByRole("button", { name: /^Unapprove / }).click();
    const unapproveDialog = page.getByRole("alertdialog");
    await expect(unapproveDialog).toBeVisible();
    await expect(unapproveDialog.getByRole("button", { name: "Unapprove", exact: true })).toBeDisabled();
    await unapproveDialog.getByRole("textbox", { name: "Admin Notes" }).fill("Needs updated totals");
    await unapproveDialog.getByRole("button", { name: "Unapprove", exact: true }).click();
    await expect(row().getByText("Needs Revision")).toBeVisible();
    await expect(card("Approved")).toHaveText(String(approved));
    await expect(card("Pending")).toHaveText(String(pending - 1));
    await expect(card("Needs Revision")).toHaveText(String(needsRevision + 1));

    // The next tests expect the report to remain in the review queue.
    await page.reload();
    await expect(row().getByText("Needs Revision")).toBeVisible();
  });

  test("the personal list excludes other users' reports even for an admin", async ({ page, request }) => {
    await page.goto("/reports");
    await expect(page.getByRole("heading", { name: "Reports", exact: true })).toBeVisible();
    await expect(rowOf(page, ACCOUNTS.user.name)).toBeVisible();
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
    await expect(rowOf(page, ACCOUNTS.user.name)).toBeVisible();
  });
});

test.describe("user, later", () => {
  test.use({ storageState: authFile("user") });

  test("revises the report sent back for revision and resubmits it to Pending", async ({ page, request }) => {
    await page.goto("/reports");
    await expect(rowOf(page, ACCOUNTS.user.name).getByText("Needs Revision")).toBeVisible();
    await page.getByRole("link", { name: "Revise Report" }).click();
    await expect(page).toHaveURL(/\/reports\/\d+\/edit$/);
    const id = page.url().match(/\/reports\/(\d+)\/edit$/)![1];
    const status = async () => (await (await request.get(`${API}/reports/${id}`)).json()).status;

    await page.getByLabel("Challenges", { exact: true }).fill("Corrected totals");
    // Longer than the 3 s autosave delay: a Needs Revision report must not resubmit itself.
    await page.waitForTimeout(3500);
    expect(await status()).toBe("needs_revision");

    await page.getByRole("button", { name: "Submit Report" }).click();
    await expect(page.getByText("Report resubmitted")).toBeVisible();
    await expect(page).toHaveURL(/\/reports$/);
    await expect(rowOf(page, ACCOUNTS.user.name).getByText("Pending")).toBeVisible();
    expect(await status()).toBe("pending");
  });

  test("deletes the report after confirming, and can file the month again", async ({ page }) => {
    await page.goto("/reports");
    await rowOf(page, ACCOUNTS.user.name).getByRole("button", { name: "Delete" }).click();
    await expect(page.getByRole("alertdialog")).toContainText(`Delete ${label}?`);
    await page.getByRole("alertdialog").getByRole("button", { name: "Delete" }).click();
    await expect(page.getByText("No reports found", { exact: true })).toBeVisible();

    await page.goto("/reports/new");
    await page.getByRole("button", { name: "Submit Report" }).click();
    await expect(page).toHaveURL(/\/reports$/);
    await expect(rowOf(page, ACCOUNTS.user.name)).toBeVisible();
  });
});
