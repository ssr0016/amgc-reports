import AxeBuilder from "@axe-core/playwright";
import type { Page } from "@playwright/test";
import { authFile, clientIp, earlierPeriod, expect, send, test } from "./support";

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
  const editor = await playwright.request.newContext({
    storageState: authFile("editor"),
    extraHTTPHeaders: { "X-Real-IP": clientIp("a11y-fixture") },
  });
  const p = earlierPeriod(3);
  const res = await send(editor, "POST", "/reports", { month: p.month, year: p.year, worker: FIXTURE_WORKER });
  expect([201, 409]).toContain(res.status()); // 409: already there from an earlier run in this database
  const list = await (await editor.get(`http://localhost:8081/api/v1/reports?worker=${FIXTURE_WORKER}`)).json();
  fixtureId = list.data[0].id;
  await editor.dispose();
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

      test("users list and new user", async ({ page }) => {
        await page.goto("/admin/users");
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
    await page.getByRole("link", { name: "Users" }).click();
    await expect(page).toHaveTitle("Users · AMGCReports");
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
      ["users", "/admin/users", /Users/],
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
        expect(await noSidewaysScroll(page)).toBe(true);
        await scan(page); // wide tables scroll inside their container, which must be reachable by keyboard
      });
    }
  });
});
