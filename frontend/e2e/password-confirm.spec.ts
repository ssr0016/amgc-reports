import { API, authFile, expect, send, test } from "./support";

// D-A14: every form that sets a password asks twice, and the server enforces it too.
// Nothing here changes a shared account's password, so the saved admin session stays valid.
test.use({ storageState: authFile("admin") });

const strong = "E2eConfirm!78901";
const created = { email: "e2e-confirm@example.com", name: "E2E Confirm" };

async function usersWith(request: import("@playwright/test").APIRequestContext, email: string) {
  return (await (await request.get(`${API}/users?limit=100&email=${email}`)).json()) as { email: string }[];
}

test("a mismatch on the change-password page is shown on Confirm password and sends nothing", async ({ page }) => {
  await page.goto("/account/password");
  await page.getByLabel("Current password").fill("anything");
  await page.getByLabel("New password").fill(strong);
  await page.getByLabel("Confirm password").fill(`${strong}x`);
  await page.getByRole("button", { name: "Change password" }).click();

  const note = page.getByText("Confirm password does not match");
  await expect(note).toBeVisible();
  await expect(page.getByLabel("Confirm password")).toBeFocused();
  await expect(page.getByLabel("Confirm password")).toHaveAttribute("aria-invalid", "true");
  await expect(page.getByLabel("Confirm password")).toHaveAttribute("aria-describedby", (await note.getAttribute("id")) ?? "");
  await expect(page.getByText("Current password is incorrect")).toHaveCount(0); // the server was never asked
  await expect(page).toHaveURL(/\/account\/password$/);
});

test("creating a user with a mismatch shows the error and creates nothing, a match creates it", async ({ page, request }) => {
  await page.goto("/admin/users/new");
  await page.getByLabel("Email").fill(created.email);
  await page.getByLabel("Name", { exact: true }).fill(created.name);
  await page.getByLabel("Password", { exact: true }).fill(strong);
  await page.getByLabel("Confirm password").fill(`${strong}x`);
  await page.getByRole("button", { name: "Create User" }).click();

  await expect(page.getByText("Confirm password does not match")).toBeVisible();
  await expect(page.getByLabel("Confirm password")).toBeFocused();
  await expect(page).toHaveURL(/\/admin\/users\/new$/);
  expect(await usersWith(request, created.email)).toEqual([]);

  await page.getByLabel("Confirm password").fill(strong);
  await page.getByRole("button", { name: "Create User" }).click();
  await expect(page).toHaveURL(/\/admin\?tab=users$/);
  expect(await usersWith(request, created.email)).toHaveLength(1);
});

test("the API refuses a missing or different confirmation with a 422 on confirm_password only", async ({ request }) => {
  const details = async (res: import("@playwright/test").APIResponse) => (await res.json()).details;

  const noConfirm = await send(request, "POST", "/admin/users", { email: "e2e-api@example.com", name: "E2E Api", password: strong });
  expect(noConfirm.status()).toBe(422);
  expect(await details(noConfirm)).toEqual({ confirm_password: "is required" });

  const different = await send(request, "POST", "/admin/users", { email: "e2e-api@example.com", name: "E2E Api", password: strong, confirm_password: `${strong}x` });
  expect(different.status()).toBe(422);
  expect(await details(different)).toEqual({ confirm_password: "does not match" });
  expect(await usersWith(request, "e2e-api@example.com")).toEqual([]);

  // A wrong current password would be a 422 on current_password; the confirmation answers first.
  const change = await send(request, "POST", "/auth/change-password", { current_password: "wrong", new_password: strong, confirm_password: `${strong}x` });
  expect(change.status()).toBe(422);
  expect(await details(change)).toEqual({ confirm_password: "does not match" });
});
