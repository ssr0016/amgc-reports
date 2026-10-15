import { ACCOUNTS, expect, test } from "./support";

test.describe("signed out", () => {
  test("a private page sends you to the login with a way back", async ({ page }) => {
    await page.goto("/reports/new");
    await expect(page).toHaveURL(/\/login\?next=%2Freports%2Fnew$/);
  });

  test("wrong password shows one generic message", async ({ page }) => {
    await page.goto("/login");
    await page.getByLabel("Email").fill(ACCOUNTS.user.email);
    await page.getByLabel("Password", { exact: true }).fill("not-the-password");
    await page.getByRole("button", { name: "Sign in" }).click();
    await expect(page.locator("form").getByRole("alert")).toHaveText("Invalid email or password.");
    await expect(page).toHaveURL(/\/login/);
  });

  test("signing in lands on the reports list and shows who you are", async ({ page }) => {
    await page.goto("/login");
    await page.getByLabel("Email").fill(ACCOUNTS.user.email);
    await page.getByLabel("Password", { exact: true }).fill(ACCOUNTS.user.password);
    await page.getByRole("button", { name: "Sign in" }).click();
    await expect(page).toHaveURL(/\/reports$/);
    await expect(page.getByText(`${ACCOUNTS.user.name} · user`)).toBeVisible();
    await expect(page.getByRole("link", { name: "Users" })).toHaveCount(0); // admin only
  });

  test("?next= is followed, but never to another site", async ({ page }) => {
    await page.goto("/login?next=/reports/new");
    await page.getByLabel("Email").fill(ACCOUNTS.user.email);
    await page.getByLabel("Password", { exact: true }).fill(ACCOUNTS.user.password);
    await page.getByRole("button", { name: "Sign in" }).click();
    await expect(page).toHaveURL(/\/reports\/new$/);
  });

  test("an off-site ?next= is ignored", async ({ page }) => {
    await page.goto("/login?next=//evil.example");
    await page.getByLabel("Email").fill(ACCOUNTS.user.email);
    await page.getByLabel("Password", { exact: true }).fill(ACCOUNTS.user.password);
    await page.getByRole("button", { name: "Sign in" }).click();
    await expect(page).toHaveURL(/localhost:3001\/reports$/);
  });

  test("the fourth attempt in a row is rate limited with a wait", async ({ page }) => {
    await page.goto("/login");
    await page.getByLabel("Email").fill(ACCOUNTS.user.email);
    await page.getByLabel("Password", { exact: true }).fill("wrong-password");
    const submit = page.getByRole("button", { name: "Sign in" });
    for (let i = 0; i < 3; i++) {
      await submit.click();
      await expect(page.locator("form").getByRole("alert")).toHaveText("Invalid email or password.");
    }
    await submit.click();
    await expect(page.locator("form").getByRole("alert")).toContainText(/Too many attempts\. Try again in \d+s\./);
    await expect(submit).toBeDisabled();
  });
});

test.describe("signed in", () => {
  // Its own login: signing out ends the server session, and the saved admin
  // session is shared with the other specs.
  test("signing out ends the session", async ({ page }) => {
    await page.goto("/login");
    await page.getByLabel("Email").fill(ACCOUNTS.admin.email);
    await page.getByLabel("Password", { exact: true }).fill(ACCOUNTS.admin.password);
    await page.getByRole("button", { name: "Sign in" }).click();
    await expect(page.getByText(`${ACCOUNTS.admin.name} · admin`)).toBeVisible();
    await page.getByRole("button", { name: "Sign out" }).click();
    await expect(page).toHaveURL(/\/login/);
    await page.goto("/reports");
    await expect(page).toHaveURL(/\/login/);
  });
});
