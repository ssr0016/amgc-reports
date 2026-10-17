import { ACCOUNTS, authFile, expect, test } from "./support";

const mainNav = (page: import("@playwright/test").Page) => page.getByRole("navigation", { name: "Main" });

test.describe("sidebar, as admin", () => {
  test.use({ storageState: authFile("admin") });

  test("lists Reports and Admin, and marks the current page", async ({ page }) => {
    await page.goto("/admin");
    await expect(mainNav(page).getByRole("link")).toHaveText(["Reports", "Admin"]);
    await expect(mainNav(page).getByRole("link", { name: "Admin" })).toHaveAttribute("aria-current", "page");
    await mainNav(page).getByRole("link", { name: "Reports" }).click();
    await expect(page).toHaveURL(/\/reports$/);
    await expect(mainNav(page).getByRole("link", { name: "Reports" })).toHaveAttribute("aria-current", "page");
    await expect(mainNav(page).getByRole("link", { name: "Admin" })).not.toHaveAttribute("aria-current", "page");
  });

  test("the old /admin/users address lands on the Users tab", async ({ page }) => {
    await page.goto("/admin/users");
    await expect(page).toHaveURL(/\/admin\?tab=users$/);
    await expect(page.getByRole("tab", { name: "Users" })).toHaveAttribute("aria-selected", "true");
    await expect(page.getByRole("link", { name: /^Edit E2E User/ })).toBeVisible();
  });

  test("the admin tab survives a refresh, and Back leaves /admin instead of stepping between tabs", async ({ page }) => {
    await page.goto("/reports");
    await page.goto("/admin");
    await page.getByRole("tab", { name: "Users" }).click();
    await expect(page).toHaveURL(/\/admin\?tab=users$/);

    await page.reload();
    await expect(page.getByRole("tab", { name: "Users" })).toHaveAttribute("aria-selected", "true");

    await page.getByRole("tab", { name: "Reports" }).click();
    await expect(page).toHaveURL(/\/admin\?tab=reports$/);
    await page.goBack();
    await expect(page).toHaveURL(/\/reports$/);
  });

  test("the avatar menu opens and leads to the password page", async ({ page }) => {
    await page.goto("/reports");
    await page.getByRole("button", { name: `${ACCOUNTS.admin.name} · admin` }).click();
    await page.getByRole("menuitem", { name: "Change password" }).click();
    await expect(page).toHaveURL(/\/account\/password$/);
    await expect(mainNav(page).locator("[aria-current]")).toHaveCount(0);
  });

  test("on a wide screen the Menu button is not shown", async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 720 });
    await page.goto("/reports");
    await expect(page.getByRole("button", { name: "Menu" })).toBeHidden();
    await expect(mainNav(page)).toBeVisible();
  });

  test("at 320 px the sidebar sits behind the Menu button, which opens it and closes after a choice", async ({ page }) => {
    await page.setViewportSize({ width: 320, height: 640 });
    await page.goto("/reports");
    const menu = page.getByRole("button", { name: "Menu" });
    await expect(mainNav(page)).toBeHidden();
    await expect(menu).toHaveAttribute("aria-expanded", "false");
    await menu.click();
    await expect(menu).toHaveAttribute("aria-expanded", "true");
    await expect(mainNav(page)).toBeVisible();
    await mainNav(page).getByRole("link", { name: "Admin" }).click();
    await expect(page).toHaveURL(/\/admin$/);
    await expect(menu).toHaveAttribute("aria-expanded", "false");
    await expect(mainNav(page)).toBeHidden();
  });
});

for (const role of ["editor", "user"] as const) {
  test.describe(`sidebar, as ${role}`, () => {
    test.use({ storageState: authFile(role) });

    test("shows only Reports", async ({ page }) => {
      await page.goto("/reports");
      await expect(mainNav(page).getByRole("link")).toHaveText(["Reports"]);
      await expect(mainNav(page).getByRole("link", { name: "Reports" })).toHaveAttribute("aria-current", "page");
    });
  });
}
