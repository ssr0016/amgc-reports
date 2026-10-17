import AxeBuilder from "@axe-core/playwright";
import type { APIRequestContext, Page, PlaywrightWorkerArgs } from "@playwright/test";
import { API, authFile, clientIp, expect, send, test } from "./support";

// D-A12. A reset ends the account's sessions and changes its password, so this uses accounts of its
// own; the saved sessions the other specs share stay valid.
const A = { email: "e2e-reset@example.com", name: "E2E Reset", password: "E2eReset!78901" };
const B = { email: "e2e-reset-axe@example.com", name: "E2E Reset Axe", password: "E2eResetAxe!78901" };
const ids = { A: 0, B: 0 };
let current = A.password; // A's password as the tests go on
let admin: APIRequestContext;

test.beforeAll(async ({ playwright }) => {
  admin = await playwright.request.newContext({
    storageState: authFile("admin"),
    extraHTTPHeaders: { "X-Real-IP": clientIp("admin-reset-setup") },
  });
  const roles: { id: number; name: string }[] = (await (await admin.get(`${API}/admin/roles?limit=100`)).json()).data;
  for (const [key, account] of [["A", A], ["B", B]] as const) {
    const res = await send(admin, "POST", "/admin/users", {
      ...account,
      confirm_password: account.password,
      role_id: roles.find((r) => r.name === "user")?.id,
      area_assignment: "",
      church_name: "",
    });
    expect(res.status(), await res.text()).toBe(201);
    ids[key] = (await res.json()).id;
  }
});

test.afterAll(async () => {
  await admin.dispose();
});

// Signs in through the API from its own address, because the login limiter is per address.
async function signIn(playwright: PlaywrightWorkerArgs["playwright"], email: string, password: string, tag: string) {
  // An empty storage state: the runner would otherwise give this context the signed-in admin's cookies,
  // and the login would renew the admin's own session.
  const ctx = await playwright.request.newContext({
    storageState: { cookies: [], origins: [] },
    extraHTTPHeaders: { "X-Real-IP": clientIp(tag) },
  });
  const res = await send(ctx, "POST", "/auth/login", { email, password });
  return { ctx, status: res.status(), body: await res.text() };
}
const meStatus = async (ctx: APIRequestContext) => (await ctx.get(`${API}/auth/me`)).status();

const editPage = (page: Page, id: number) => page.goto(`/admin/users/${id}/edit`);
const section = (page: Page) => page.getByRole("group", { name: "Reset password" });
const openDialog = async (page: Page) => {
  await section(page).getByRole("button", { name: "Reset password" }).click();
  const dialog = page.getByRole("alertdialog");
  await expect(dialog).toBeVisible();
  return dialog;
};

test.describe("as admin", () => {
  test.use({ storageState: authFile("admin") });
  test.describe.configure({ mode: "serial" });

  test("a mismatch in the dialog shows the error and resets nothing", async ({ page, playwright }) => {
    await editPage(page, ids.A);
    const dialog = await openDialog(page);
    await dialog.getByLabel("New password").fill("E2eNever!Used99");
    await dialog.getByLabel("Confirm password").fill("E2eNever!Used98");
    await dialog.getByRole("button", { name: "Reset password" }).click();

    await expect(dialog.getByText("Confirm password does not match")).toBeVisible();
    await expect(dialog.getByLabel("Confirm password")).toBeFocused();
    await dialog.getByRole("button", { name: "Cancel" }).click();
    await expect(page.getByRole("alertdialog")).toHaveCount(0);

    const still = await signIn(playwright, A.email, current, "reset-mismatch");
    expect(still.status, still.body).toBe(200);
    await still.ctx.dispose();
  });

  test("Generate, Copy and Reset: the server accepts the password, the old one is refused, the open session ends", async ({ page, context, playwright }) => {
    await context.grantPermissions(["clipboard-read", "clipboard-write"], { origin: "http://localhost:3001" });
    const open = await signIn(playwright, A.email, current, "reset-open-session");
    expect(open.status, open.body).toBe(200);
    expect(await meStatus(open.ctx)).toBe(200);

    await editPage(page, ids.A);
    const dialog = await openDialog(page);
    await dialog.getByRole("button", { name: "Generate random password" }).click();
    const generated = await dialog.getByLabel("New password").inputValue();
    expect(generated).toHaveLength(20);
    expect(generated).not.toMatch(/[0O1lI]/);
    await expect(dialog.getByLabel("Confirm password")).toHaveValue(generated);
    await expect(dialog.getByLabel("New password")).toHaveAttribute("type", "text");

    await dialog.getByRole("button", { name: "Copy password" }).click();
    await expect(page.getByText("Copied")).toBeVisible();
    expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(generated);

    await dialog.getByRole("button", { name: "Reset password" }).click();
    await expect(dialog.getByText("This password will not be shown again.")).toBeVisible();
    await expect(dialog.getByLabel("Password", { exact: true })).toHaveValue(generated);
    await expect(page.getByText("Password reset for E2E Reset")).toBeVisible();
    await dialog.getByRole("button", { name: "Close" }).click();
    await expect(page.getByRole("alertdialog")).toHaveCount(0);

    const old = await signIn(playwright, A.email, current, "reset-old-password");
    expect(old.status).toBe(401);
    expect(old.body).toContain("invalid email or password");
    await old.ctx.dispose();
    const fresh = await signIn(playwright, A.email, generated, "reset-new-password");
    expect(fresh.status, fresh.body).toBe(200);
    await fresh.ctx.dispose();
    expect(await meStatus(open.ctx), "an open session of the account survived the reset").toBe(401);
    await open.ctx.dispose();
    current = generated;
  });

  test("a locked-out account signs in with the new password", async ({ playwright }) => {
    // Five wrong passwords lock the account. Each comes from its own address, because the login limiter is per address.
    for (let i = 0; i < 5; i++) {
      const wrong = await signIn(playwright, A.email, "not-the-password", `reset-lock-${i}`);
      expect(wrong.status, `attempt ${i + 1}: ${wrong.body}`).toBe(401);
      await wrong.ctx.dispose();
    }
    const locked = await signIn(playwright, A.email, current, "reset-locked");
    expect(locked.status, "the right password should be refused while locked").toBe(403);
    expect(locked.body.toLowerCase()).toContain("account locked");
    await locked.ctx.dispose();

    const next = "E2eReset!After-Lock7";
    const res = await send(admin, "POST", `/admin/users/${ids.A}/reset-password`, { new_password: next, confirm_password: next });
    expect(res.status(), await res.text()).toBe(204);

    const again = await signIn(playwright, A.email, next, "reset-after-lock");
    expect(again.status, again.body).toBe(200);
    await again.ctx.dispose();
    current = next;
  });

  test("on your own page the button is off and the API says 403", async ({ page, request }) => {
    const me = (await (await request.get(`${API}/auth/me`)).json()).user_id;
    await editPage(page, me);
    const group = section(page);
    await expect(group.getByRole("button", { name: "Reset password" })).toBeDisabled();
    await expect(group.getByText("You cannot reset your own password. Use the Change Password page instead.")).toBeVisible();

    const res = await send(request, "POST", `/admin/users/${me}/reset-password`, { new_password: "E2eSelf!Reset-77x", confirm_password: "E2eSelf!Reset-77x" });
    expect(res.status()).toBe(403);
    expect((await res.json()).message).toBe("You cannot reset your own password. Use the Change Password page instead.");
  });

  test("the API refuses a weak password and a mismatch with a 422 and resets nothing", async ({ playwright }) => {
    const weak = await send(admin, "POST", `/admin/users/${ids.A}/reset-password`, { new_password: "password", confirm_password: "password" });
    expect(weak.status()).toBe(422);
    expect(Object.keys((await weak.json()).details)).toEqual(["new_password"]);
    const different = await send(admin, "POST", `/admin/users/${ids.A}/reset-password`, { new_password: "E2eReset!Other-99z", confirm_password: "E2eReset!Other-99y" });
    expect(different.status()).toBe(422);
    expect((await different.json()).details).toEqual({ confirm_password: "does not match" });
    const missing = await send(admin, "POST", "/admin/users/999999/reset-password", { new_password: "E2eReset!Other-99z", confirm_password: "E2eReset!Other-99z" });
    expect(missing.status()).toBe(404);

    const still = await signIn(playwright, A.email, current, "reset-422");
    expect(still.status, still.body).toBe(200);
    await still.ctx.dispose();
  });
});

test.describe("as a plain user", () => {
  test.use({ storageState: authFile("user") });

  test("the API says 403 and nothing changes", async ({ request, playwright }) => {
    const res = await send(request, "POST", `/admin/users/${ids.A}/reset-password`, { new_password: "E2eSneaky!Reset-1x", confirm_password: "E2eSneaky!Reset-1x" });
    expect(res.status()).toBe(403);
    const still = await signIn(playwright, A.email, current, "reset-non-admin");
    expect(still.status, still.body).toBe(200);
    await still.ctx.dispose();
  });
});

// WCAG 2.x A and AA plus axe's best practices, as in a11y.spec.ts.
const TAGS = ["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa", "best-practice"];
async function scan(page: Page) {
  const { violations } = await new AxeBuilder({ page }).withTags(TAGS).exclude("nextjs-portal").analyze();
  expect(violations.map((v) => `${v.id} (${v.impact}): ${v.nodes.map((n) => n.target.join(" ")).join(" | ")}`)).toEqual([]);
}

for (const scheme of ["light", "dark"] as const) {
  test.describe(`axe, ${scheme} theme`, () => {
    test.use({ colorScheme: scheme, storageState: authFile("admin") });

    test("the Reset password section and the dialog in its form, generated and done states", async ({ page }) => {
      await editPage(page, ids.B);
      await expect(section(page)).toBeVisible();
      await scan(page);

      const dialog = await openDialog(page);
      await scan(page);
      await dialog.getByRole("button", { name: "Reset password" }).click();
      await expect(dialog.getByText("New password is required")).toBeVisible();
      await scan(page); // the error state

      await dialog.getByRole("button", { name: "Generate random password" }).click();
      await expect(dialog.getByLabel("New password")).toHaveAttribute("type", "text");
      await scan(page);

      await dialog.getByRole("button", { name: "Reset password" }).click();
      await expect(dialog.getByText("This password will not be shown again.")).toBeVisible();
      // The "Password reset" toast fades in; axe reads its half-transparent text as low contrast. Scan once it has gone.
      await expect(page.locator("[data-sonner-toast]")).toHaveCount(0, { timeout: 15_000 });
      await scan(page);
    });
  });
}
