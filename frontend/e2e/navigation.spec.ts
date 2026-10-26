import { ACCOUNTS, authFile, expect, test } from "./support";

const mainNav = (page: import("@playwright/test").Page) => page.getByRole("navigation", { name: "Main" });

test.describe("sidebar, as admin", () => {
  test.use({ storageState: authFile("admin") });

  test("lists reports, admin sections and settings, and marks the current page", async ({ page }) => {
    await page.goto("/admin");
    await expect(mainNav(page).getByRole("link")).toHaveText(["Reports", "Reports", "Tracker", "Users", "Roles", "Logs", "Profile", "Change Password"]);
    await expect(mainNav(page).getByRole("link", { name: "Admin Reports", exact: true })).toHaveAttribute("aria-current", "page");
    await mainNav(page).getByRole("link", { name: "Reports", exact: true }).click();
    await expect(page).toHaveURL(/\/reports$/);
    await expect(mainNav(page).getByRole("link", { name: "Reports", exact: true })).toHaveAttribute("aria-current", "page");
    await expect(mainNav(page).getByRole("link", { name: "Admin Reports", exact: true })).not.toHaveAttribute("aria-current", "page");
  });

  test("the old /admin/users address lands on the Users section", async ({ page }) => {
    await page.goto("/admin/users");
    await expect(page).toHaveURL(/\/admin\?tab=users$/);
    await expect(page.getByRole("link", { name: "Admin Users", exact: true })).toHaveAttribute("aria-current", "page");
    await expect(page.getByRole("link", { name: /^Edit E2E User/ })).toBeVisible();
  });

  test("the admin section survives a refresh, and Back leaves /admin instead of stepping between sections", async ({ page }) => {
    await page.goto("/reports");
    await page.goto("/admin");
    await page.getByRole("link", { name: "Admin Users", exact: true }).click();
    await expect(page).toHaveURL(/\/admin\?tab=users$/);

    await page.reload();
    await expect(page.getByRole("link", { name: "Admin Users", exact: true })).toHaveAttribute("aria-current", "page");

    await page.getByRole("link", { name: "Admin Reports", exact: true }).click();
    await expect(page).toHaveURL(/\/admin\?tab=reports$/);
    await page.goBack();
    await expect(page).toHaveURL(/\/reports$/);
  });

  test("the avatar menu opens and leads to the password page", async ({ page }) => {
    await page.goto("/reports");
    await page.getByRole("button", { name: `${ACCOUNTS.admin.name} · admin` }).click();
    await page.getByRole("menuitem", { name: "Change password" }).click();
    await expect(page).toHaveURL(/\/account\/password$/);
    await expect(mainNav(page).getByRole("link", { name: "Change Password" })).toHaveAttribute("aria-current", "page");
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
    const menu = page.getByRole("button", { name: "Menu", includeHidden: true });
    await expect(mainNav(page)).toBeHidden();
    await expect(menu).toHaveAttribute("aria-expanded", "false");
    await menu.click();
    await expect(menu).toHaveAttribute("aria-expanded", "true");
    const mobileNav = page.getByRole("navigation", { name: "Mobile" });
    await expect(mobileNav).toBeVisible();
    await mobileNav.getByRole("link", { name: "Admin Reports", exact: true }).click();
    await expect(page).toHaveURL(/\/admin\?tab=reports$/);
    await expect(menu).toHaveAttribute("aria-expanded", "false");
    await expect(mainNav(page)).toBeHidden();
    await expect(mobileNav).toHaveCount(0);
  });
});

for (const role of ["editor", "user"] as const) {
  test.describe(`sidebar, as ${role}`, () => {
    test.use({ storageState: authFile(role) });

    test("shows reports and account settings", async ({ page }) => {
      await page.goto("/reports");
      await expect(mainNav(page).getByRole("link")).toHaveText(["Reports", "Profile", "Change Password"]);
      await expect(mainNav(page).getByRole("link", { name: "Reports", exact: true })).toHaveAttribute("aria-current", "page");
    });
  });
}
