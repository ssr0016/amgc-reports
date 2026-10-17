import type { APIRequestContext, Page } from "@playwright/test";
import { API, authFile, clientIp, expect, send, test } from "./support";

// A status change ends that account's sessions, so this uses an account of its own;
// the saved sessions the other specs share stay valid.
const account = { email: "e2e-status@example.com", name: "E2E Status", password: "E2eState!78901" };
let userId = 0;
let admin: APIRequestContext;

test.beforeAll(async ({ playwright }) => {
  admin = await playwright.request.newContext({
    storageState: authFile("admin"),
    extraHTTPHeaders: { "X-Real-IP": clientIp("user-status-setup") },
  });
  const roles: { id: number; name: string }[] = (await (await admin.get(`${API}/admin/roles?limit=100`)).json()).data;
  const res = await send(admin, "POST", "/admin/users", {
    ...account,
    role_id: roles.find((r) => r.name === "user")?.id,
    area_assignment: "",
    church_name: "",
  });
  expect(res.status(), await res.text()).toBe(201);
  userId = (await res.json()).id;
});

test.afterAll(async () => {
  await admin.dispose();
});

const setStatus = async (status: string) => {
  const res = await send(admin, "PUT", `/admin/users/${userId}/status`, { status });
  expect(res.status(), await res.text()).toBe(200);
};

async function signIn(page: Page, password = account.password) {
  await page.goto("/login");
  await page.getByLabel("Email").fill(account.email);
  await page.getByLabel("Password", { exact: true }).fill(password);
  await page.getByRole("button", { name: "Sign in" }).click();
}

test.describe("a non-active account", () => {
  test.use({ storageState: { cookies: [], origins: [] } });
  test.describe.configure({ mode: "serial" });
  test.beforeEach(() => setStatus("active"));

  test("is refused at sign-in with its own message, and signs in again once Active", async ({ page }) => {
    await setStatus("inactive");
    await signIn(page);
    await expect(page.getByText("Your account is inactive. Contact your admin.")).toBeVisible();
    await expect(page).toHaveURL(/\/login/);

    await setStatus("active");
    await signIn(page);
    await expect(page).toHaveURL(/\/reports$/);
  });

  for (const [status, message] of [
    ["disciplinary", "Your account is under review. Contact your admin."],
    ["deceased", "This account is closed."],
  ] as const) {
    test(`${status} gets its own message`, async ({ page }) => {
      await setStatus(status);
      await signIn(page);
      await expect(page.getByText(message)).toBeVisible();
    });
  }

  test("a wrong password on a closed account is still the generic message", async ({ page }) => {
    await setStatus("deceased");
    await signIn(page, "not-the-password");
    await expect(page.getByText("Invalid email or password.")).toBeVisible();
  });

  test("an open session stops working right after the admin sets the account inactive", async ({ page }) => {
    await signIn(page);
    await expect(page).toHaveURL(/\/reports$/);

    await setStatus("inactive");
    await page.reload();

    await expect(page).toHaveURL(/\/login/);
  });
});

test.describe("as admin", () => {
  test.use({ storageState: authFile("admin") });
  test.describe.configure({ mode: "serial" });
  test.beforeEach(() => setStatus("active"));

  test("the Users tab shows the badge and the Status filter narrows the list", async ({ page }) => {
    await setStatus("inactive");
    await page.goto("/admin");
    await page.getByRole("tab", { name: "Users" }).click();
    const panel = page.getByRole("tabpanel", { name: "Users" });

    const row = panel.getByRole("row", { name: /E2E Status/ });
    await expect(row.getByTestId("user-status")).toHaveText("Inactive");
    await expect(panel.getByRole("row", { name: /E2E Admin/ })).toBeVisible();

    await panel.getByLabel("Status").selectOption({ label: "Inactive" });
    await expect(row).toBeVisible();
    await expect(panel.getByRole("row", { name: /E2E Admin/ })).toHaveCount(0);
  });

  test("the Edit page changes the status after a confirmation", async ({ page }) => {
    await page.goto(`/admin/users/${userId}/edit`);
    const group = page.getByRole("group", { name: "Status" });
    await group.getByLabel("Status").selectOption({ label: "Disciplinary" });
    await group.getByRole("button", { name: "Update status" }).click();
    await page.getByRole("alertdialog").getByRole("button", { name: "Change status" }).click();

    await expect(page.getByText("E2E Status is now Disciplinary")).toBeVisible();
    const res = await admin.get(`${API}/admin/users/${userId}`);
    expect((await res.json()).status).toBe("disciplinary");
  });

  test("an admin cannot change their own status: the select is off and the API says 403", async ({ page, request }) => {
    const me = (await (await request.get(`${API}/auth/me`)).json()).user_id;

    await page.goto(`/admin/users/${me}/edit`);
    const group = page.getByRole("group", { name: "Status" });
    await expect(group.getByLabel("Status")).toBeDisabled();
    await expect(group.getByText("You cannot change your own status.")).toBeVisible();

    const res = await send(request, "PUT", `/admin/users/${me}/status`, { status: "inactive" });
    expect(res.status()).toBe(403);
  });
});
