import AxeBuilder from "@axe-core/playwright";
import type { Page } from "@playwright/test";
import { API, authFile, clientIp, earlierPeriod, expect, previousPeriod, send, test } from "./support";

// WCAG 2.x A and AA plus axe's best practices. Axe cannot judge everything (focus
// visibility, reading order, real screen reader output), so keyboard flows are
// tested below and the rest is recorded as a known limit in 08.
const TAGS = ["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa", "best-practice"];

async function scan(page: Page) {
  const { violations } = await new AxeBuilder({ page })
    .withTags(TAGS)
    .exclude("nextjs-portal") // the dev server's overlay, not the app
    .analyze();
  // One readable line per rule, so a failure says what and where.
  expect(
    violations.map((v) => `${v.id} (${v.impact}): ${v.nodes.map((n) => n.target.join(" ")).join(" | ")}`),
  ).toEqual([]);
}

// A report by someone else, so the list has a row to scan, the user sees a read-only form,
// and the admin can open the delete dialog without touching the other specs' data.
const FIXTURE_WORKER = "A11y Fixture";
let fixtureId = 0;

test.beforeAll(async ({ playwright }) => {
  const admin = await playwright.request.newContext({
    storageState: authFile("admin"),
    extraHTTPHeaders: { "X-Real-IP": clientIp("a11y-fixture") },
  });
  const author = await playwright.request.newContext({
    extraHTTPHeaders: { "X-Real-IP": clientIp("a11y-author") },
  });
  try {
    const roles: { id: number; name: string }[] = (await (await admin.get(`${API}/admin/roles?limit=100`)).json()).data;
    const account = { email: "e2e-a11y-author@example.com", name: "E2E A11y Author", password: "E2eA11y!23456" };
    const created = await send(admin, "POST", "/admin/users", {
      ...account, confirm_password: account.password, role_id: roles.find((r) => r.name === "user")!.id,
      area_assignment: "", church_name: "",
    });
    expect([201, 409], await created.text()).toContain(created.status());
    const login = await send(author, "POST", "/auth/login", { email: account.email, password: account.password });
    expect(login.status(), await login.text()).toBe(200);
    const existing = await (await admin.get(`${API}/reports?worker=${FIXTURE_WORKER}`)).json();
    if (existing.data.length) {
      fixtureId = existing.data[0].id;
    } else {
      const res = await send(author, "POST", "/reports", { ...previousPeriod(), worker: FIXTURE_WORKER });
      expect(res.status(), await res.text()).toBe(201);
      fixtureId = (await res.json()).id;
    }
    // Keep this fixture outside the month used by the report-creation tests.
    const moved = await send(admin, "PUT", `/reports/${fixtureId}`, { ...earlierPeriod(3), worker: FIXTURE_WORKER });
    expect(moved.status(), await moved.text()).toBe(200);
  } finally {
    await author.dispose();
    await admin.dispose();
  }
});

for (const scheme of ["light", "dark"] as const) {
  test.describe(`axe, ${scheme} theme`, () => {
    test.use({ colorScheme: scheme });

    test("login", async ({ page }) => {
      await page.goto("/login");
      await expect(page.getByRole("heading", { name: "Sign in" })).toBeVisible();
      await scan(page);
    });

    test.describe("as admin", () => {
      test.use({ storageState: authFile("admin") });

      test("admin panel", async ({ page }) => {
        await page.goto("/admin");
        await expect(page.getByRole("tab", { name: "Reports" })).toBeVisible();
        await expect(page.getByRole("region", { name: "Statistics" })).toBeVisible(); // the cards, once /admin/stats answered
        await expect(page.getByText("Loading…")).toHaveCount(0); // and the reports list
        await scan(page);
      });

      test("admin panel, Logs tab, with a row expanded", async ({ page }) => {
        await page.goto("/admin?tab=logs");
        const panel = page.getByRole("tabpanel", { name: "Logs" });
        await expect(panel.getByText(/total logs$/)).toBeVisible(); // the setup's own sign-ins are always there
        await panel.getByRole("button", { name: /View details/ }).first().click();
        await scan(page);
      });

      test("admin panel, Tracker tab", async ({ page }) => {
        await page.goto("/admin?tab=tracker");
        const panel = page.getByRole("tabpanel", { name: "Tracker" });
        await expect(panel.getByText(/of \d+ workers$/)).toBeVisible(); // the setup's e2e user is always tracked
        await scan(page);
      });

      test("admin panel, Users tab", async ({ page }) => {
        await page.goto("/admin");
        await page.getByRole("tab", { name: "Users" }).click();
        const panel = page.getByRole("tabpanel", { name: "Users" });
        await expect(panel.getByText("You", { exact: true })).toBeVisible(); // the signed-in admin's own row
        await scan(page);
        await panel.getByLabel("Role").selectOption({ label: "admin" });
        await expect(panel.getByText("You", { exact: true })).toBeVisible();
        await scan(page);
      });

      test("admin panel, Roles tab, and the role form", async ({ page }) => {
        await page.goto("/admin?tab=roles");
        const panel = page.getByRole("tabpanel", { name: "Roles" });
        await expect(panel.getByText("System", { exact: true }).first()).toBeVisible();
        await scan(page);
        await page.goto("/admin/roles/new");
        await expect(page.getByRole("group", { name: "Reports" })).toBeVisible();
        await scan(page);
      });

      test("edit user, with the Status section", async ({ page, request }) => {
        const users = await (await request.get(`${API}/users?limit=100&email=e2e-user`)).json();
        await page.goto(`/admin/users/${users[0].id}/edit`);
        await expect(page.getByRole("group", { name: "Status" })).toBeVisible();
        await scan(page);
      });

      test("reports list, with a row", async ({ page }) => {
        await page.goto(`/reports?month=&year=${earlierPeriod(3).year}`);
        await expect(page.getByRole("row", { name: new RegExp(FIXTURE_WORKER) })).toBeVisible();
        await scan(page);
      });

      test("delete dialog", async ({ page }) => {
        await page.goto(`/reports?month=&year=${earlierPeriod(3).year}`);
        await page.getByRole("button", { name: /^Delete .*A11y Fixture/ }).click();
        await expect(page.getByRole("alertdialog")).toBeVisible();
        await scan(page);
      });

      test("change password and new user, with a confirmation error", async ({ page }) => {
        await page.goto("/account/password");
        await page.getByLabel("Current password").fill("anything");
        await page.getByLabel("New password").fill("E2eConfirm!78901");
        await page.getByLabel("Confirm password").fill("E2eConfirm!78902");
        await page.getByRole("button", { name: "Change password" }).click();
        await expect(page.getByText("Confirm password does not match")).toBeVisible();
        await scan(page);

        await page.goto("/admin/users/new");
        await page.getByLabel("Email").fill("e2e-a11y-confirm@example.com");
        await page.getByLabel("Name", { exact: true }).fill("E2E A11y");
        await page.getByLabel("Password", { exact: true }).fill("E2eConfirm!78901");
        await page.getByLabel("Confirm password").fill("E2eConfirm!78902");
        await page.getByRole("button", { name: "Create User" }).click();
        await expect(page.getByText("Confirm password does not match")).toBeVisible();
        await scan(page);
      });

      test("users list and new user", async ({ page }) => {
        await page.goto("/admin/users"); // redirects to the Users tab of /admin
        await expect(page.getByRole("link", { name: /^Edit E2E User/ })).toBeVisible();
        await scan(page);
        await page.goto("/admin/users/new");
        await expect(page.getByRole("heading", { name: "New user" })).toBeVisible();
        await scan(page);
      });
    });

    test.describe("as user", () => {
      test.use({ storageState: authFile("user") });

      test("new report form", async ({ page }) => {
        await page.goto("/reports/new");
        await expect(page.getByLabel("Worker Name")).toBeVisible();
        await scan(page);
      });

      test("another user's report is read-only", async ({ page }) => {
        await page.goto(`/reports/${fixtureId}/edit`);
        await expect(page.getByRole("heading", { name: "View report" })).toBeVisible();
        // D-18: a reader without write_any sees the period and counts, not the free text or tithes.
        await expect(page.getByText("Restricted")).toHaveCount(5);
        await scan(page);
      });
    });
  });
}

test.describe("page titles", () => {
  test.use({ storageState: authFile("admin") });

  test("each page names itself, and a client-side navigation updates it", async ({ page }) => {
    await page.goto("/reports");
    await expect(page).toHaveTitle("Reports · AMGCReports");
    await page.getByRole("navigation", { name: "Main" }).getByRole("link", { name: "Admin" }).click();
    await expect(page).toHaveTitle("Admin · AMGCReports");
    await page.getByRole("tab", { name: "Users" }).click();
    await page.getByRole("link", { name: "New user" }).click();
    await expect(page).toHaveTitle("New user · AMGCReports");
    await page.goto("/reports/new");
    await expect(page).toHaveTitle("New report · AMGCReports");
  });

  test("the login page has its own title", async ({ browser }) => {
    const page = await (await browser.newPage());
    await page.goto("/login");
    await expect(page).toHaveTitle("Sign in · AMGCReports");
    await page.close();
  });
});

test.describe("keyboard", () => {
  test.describe("signed in as admin", () => {
    test.use({ storageState: authFile("admin") });

    test("the skip link is the first tab stop, shows on focus and moves focus to the main content", async ({ page }) => {
      await page.goto("/reports");
      await expect(page.getByRole("button", { name: "Sign out" })).toBeVisible();
      await page.keyboard.press("Tab");
      const skip = page.getByRole("link", { name: "Skip to main content" });
      await expect(skip).toBeFocused();
      await expect(skip).toBeInViewport();
      await page.keyboard.press("Enter");
      await expect(page.locator("main#main-content")).toBeFocused();
      await page.keyboard.press("Tab"); // the next stop is inside the page, not back in the header
      expect(await page.evaluate(() => document.activeElement?.closest("main") !== null)).toBe(true);
    });

    test("the delete dialog traps focus, closes on Escape and gives focus back", async ({ page }) => {
      await page.goto(`/reports?month=&year=${earlierPeriod(3).year}`);
      const open = page.getByRole("button", { name: /^Delete .*A11y Fixture/ });
      await open.focus();
      await page.keyboard.press("Enter");
      const dialog = page.getByRole("alertdialog");
      await expect(dialog).toBeVisible();
      await expect(dialog.getByRole("button", { name: "Cancel" })).toBeFocused(); // the safe choice first

      for (let i = 0; i < 4; i++) {
        await page.keyboard.press("Tab");
        expect(await page.evaluate(() => document.activeElement?.closest('[role="alertdialog"]') !== null)).toBe(true);
      }
      await page.keyboard.press("Escape");
      await expect(dialog).toBeHidden();
      await expect(open).toBeFocused();
    });
  });

  test.describe("as user", () => {
    test.use({ storageState: authFile("user") });

    test("a failed submit with the keyboard lands on the first problem", async ({ page }) => {
      await page.goto("/reports/new");
      const bad = page.getByLabel("Tithes & Offering, week 1");
      await bad.focus();
      await page.keyboard.type("1.234");
      await page.keyboard.press("Tab");
      await expect(page.getByText("Must be a valid amount (0.00)", { exact: true })).toBeVisible();
      await expect(bad).toHaveAttribute("aria-describedby", /err-tithes_offering-week1/);

      await page.getByLabel("Worker Name").focus();
      await page.keyboard.press("Enter"); // implicit submit from a text field
      await expect(bad).toBeFocused();
      await expect(page).toHaveURL(/\/reports\/new$/);
    });
  });
});

test.describe("reduced motion", () => {
  test.use({ storageState: authFile("admin") });

  async function dialogAnimationSeconds(page: Page) {
    await page.goto(`/reports?month=&year=${earlierPeriod(3).year}`);
    await page.getByRole("button", { name: /^Delete .*A11y Fixture/ }).click();
    const dialog = page.getByRole("alertdialog");
    await expect(dialog).toBeVisible();
    return dialog.evaluate((el) => Math.max(...getComputedStyle(el).animationDuration.split(",").map(parseFloat)));
  }

  test("animations run normally by default", async ({ page }) => {
    await page.emulateMedia({ reducedMotion: "no-preference" });
    expect(await dialogAnimationSeconds(page)).toBeGreaterThan(0.05);
  });

  test("and are effectively off when the user asks for reduced motion (WCAG 2.3.3)", async ({ page }) => {
    await page.emulateMedia({ reducedMotion: "reduce" });
    expect(await dialogAnimationSeconds(page)).toBeLessThan(0.001);
  });
});

test.describe("reflow at 320 px (WCAG 1.4.10)", () => {
  test.use({ viewport: { width: 320, height: 640 } });

  // Tables may scroll inside their own container; the page itself must not scroll sideways.
  const noSidewaysScroll = (page: Page) =>
    page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth);

  test("login", async ({ page }) => {
    await page.goto("/login");
    await expect(page.getByRole("heading", { name: "Sign in" })).toBeVisible();
    expect(await noSidewaysScroll(page)).toBe(true);
  });

  test.describe("signed in", () => {
    test.use({ storageState: authFile("admin") });

    for (const [name, path, ready] of [
      ["reports list", `/reports?month=&year=${earlierPeriod(3).year}`, /Reports/],
      ["new report", "/reports/new", /New report/],
      ["admin", "/admin", /Admin/],
      ["tracker tab", "/admin?tab=tracker", /Admin/],
      ["users tab", "/admin?tab=users", /Admin/],
      ["logs tab", "/admin?tab=logs", /Admin/],
      ["roles tab", "/admin?tab=roles", /Admin/],
      ["new role", "/admin/roles/new", /New role/],
      ["new user", "/admin/users/new", /New user/],
    ] as const) {
      test(name, async ({ page }) => {
        await page.goto(path);
        await expect(page.getByRole("heading", { level: 1, name: ready })).toBeVisible();
        await expect(page.getByRole("button", { name: "Sign out" })).toBeVisible();
        if (path === "/admin") {
          await expect(page.getByRole("region", { name: "Statistics" })).toBeVisible();
          await expect(page.getByText("Loading…")).toHaveCount(0); // the reports list too
        }
        if (path === "/admin?tab=logs") await expect(page.getByText(/total logs$/)).toBeVisible();
        if (path === "/admin?tab=tracker") await expect(page.getByText(/of \d+ workers$/)).toBeVisible();
        if (path === "/admin?tab=roles") await expect(page.getByText("System", { exact: true }).first()).toBeVisible();
        if (path === "/admin/roles/new") await expect(page.getByRole("group", { name: "Reports" })).toBeVisible();
        expect(await noSidewaysScroll(page)).toBe(true);
        await scan(page); // wide tables scroll inside their container, which must be reachable by keyboard
      });
    }

    test("admin, with the sidebar menu open", async ({ page }) => {
      await page.goto("/reports");
      await page.getByRole("button", { name: "Menu" }).click();
      await expect(page.getByRole("navigation", { name: "Main" })).toBeVisible();
      expect(await noSidewaysScroll(page)).toBe(true);
      await scan(page);
    });
  });
});
