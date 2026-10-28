import { API, authFile, clientIp, expect, send, signOut, test } from "./support";

// Changing a password ends the account's other sessions, so this uses an account of its
// own; the saved sessions the other specs share stay valid.
const account = { email: "e2e-password@example.com", name: "E2E Password", password: "E2ePass!23456" };
const next = "E2eChanged!78901";

test.beforeAll(async ({ playwright }) => {
  const admin = await playwright.request.newContext({
    storageState: authFile("admin"),
    extraHTTPHeaders: { "X-Forwarded-For": clientIp("account-setup") },
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

async function signIn(page: import("@playwright/test").Page, password: string) {
  await page.goto("/login");
  await page.getByLabel("Email").fill(account.email);
  await page.getByLabel("Password", { exact: true }).fill(password);
  await page.getByRole("button", { name: "Sign in" }).click();
}

test.describe.configure({ mode: "serial" });

test("a wrong current password is shown on that field and keeps you signed in", async ({ page }) => {
  await signIn(page, account.password);
  await page.getByRole("button", { name: `${account.name} · user` }).click();
  await page.getByRole("menuitem", { name: "Change Password" }).click();
  await expect(page).toHaveURL(/\/account\/password$/);
  await page.getByLabel("Current password").fill("not-the-password");
  await page.getByLabel("New password").fill(next);
  await page.getByLabel("Confirm password").fill(next);
  await page.getByRole("button", { name: "Change Password" }).click();
  await expect(page.getByText("Current password is incorrect")).toBeVisible();
  await expect(page.getByLabel("Current password")).toBeFocused();
  await expect(page.getByText(`${account.name} · user`)).toBeVisible();
});

test("changing the password keeps this session and the new password signs in", async ({ page }) => {
  await signIn(page, account.password);
  await expect(page).toHaveURL(/\/reports$/); // goto before the login lands would drop the session
  await page.goto("/account/password");
  await page.getByLabel("Current password").fill(account.password);
  await page.getByLabel("New password").fill(next);
  await page.getByLabel("Confirm password").fill(next);
  await page.getByRole("button", { name: "Change Password" }).click();
  await expect(page.getByText("Password changed")).toBeVisible();
  await expect(page.getByLabel("Current password")).toHaveValue("");
  await expect(page.getByLabel("Confirm password")).toHaveValue("");

  await page.goto("/reports");
  await expect(page.getByText(`${account.name} · user`)).toBeVisible();

  await signOut(page);
  await expect(page).toHaveURL(/\/login$/);
  await signIn(page, account.password);
  await expect(page.locator("form").getByRole("alert")).toHaveText("Invalid email or password.");
  await signIn(page, next);
  await expect(page).toHaveURL(/\/reports$/);
});
