import { API, authFile, clientIp, expect, send, signOut, test } from "./support";

// A refresh token is single use, so this spec signs in as an account of its own: the
// saved sign-ins other specs share must not be rotated under them.
const account = { email: "e2e-refresh@example.com", name: "E2E Refresh", password: "E2eRefresh!23456" };

test.beforeAll(async ({ playwright }) => {
  const admin = await playwright.request.newContext({
    storageState: authFile("admin"),
    extraHTTPHeaders: { "X-Forwarded-For": clientIp("refresh-setup") },
  });
  const roles: { id: number; name: string }[] = (await (await admin.get(`${API}/admin/roles?limit=100`)).json()).data;
  const res = await send(admin, "POST", "/admin/users", {
    ...account,
    confirm_password: account.password,
    role_id: roles.find((r) => r.name === "user")?.id,
    area_assignment: "",
    church_name: "",
  });
  expect(res.status(), await res.text()).toBe(201);
  await admin.dispose();
});

test.use({ storageState: { cookies: [], origins: [] } });
test.describe.configure({ mode: "serial" });

async function signIn(page: import("@playwright/test").Page) {
  await page.goto("/login");
  await page.getByLabel("Email").fill(account.email);
  await page.getByLabel("Password", { exact: true }).fill(account.password);
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page).toHaveURL(/\/reports$/);
}

const cookieNamed = async (context: import("@playwright/test").BrowserContext, name: string) =>
  (await context.cookies()).find((c) => c.name === name);

test("an expired access token is refreshed without signing the user out", async ({ page, context }) => {
  await signIn(page);
  const before = await cookieNamed(context, "refresh_token");
  expect(before, "login sets the refresh cookie").toBeDefined();
  expect(before?.httpOnly).toBe(true);
  expect(before?.path).toBe("/api/v1/auth");

  // What the browser does when the access token's Max-Age runs out: the cookie is gone.
  await context.clearCookies({ name: "access_token" });
  expect(await cookieNamed(context, "access_token")).toBeUndefined();

  await page.reload();

  await expect(page).toHaveURL(/\/reports$/);
  await expect(page.getByText(`${account.name} · user`)).toBeVisible();
  expect(await cookieNamed(context, "access_token"), "the refresh set a new access cookie").toBeDefined();
  const after = await cookieNamed(context, "refresh_token");
  expect(after?.value, "the refresh token rotates").not.toBe(before?.value);
});

test("signing out ends the sign-in: the refresh token no longer works", async ({ page, context }) => {
  await signIn(page);
  const refresh = (await cookieNamed(context, "refresh_token"))?.value;

  await signOut(page);
  await expect(page).toHaveURL(/\/login/);

  // Put the old refresh cookie back, as a thief who copied it would.
  await context.addCookies([
    { name: "refresh_token", value: refresh!, domain: "localhost", path: "/api/v1/auth", httpOnly: true, sameSite: "Lax" },
  ]);
  await page.goto("/reports");
  await expect(page).toHaveURL(/\/login/);
});
