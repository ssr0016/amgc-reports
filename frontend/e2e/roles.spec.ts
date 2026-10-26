import type { APIRequestContext } from "@playwright/test";
import { ACCOUNTS, API, authFile, expect, send, test } from "./support";

const ROLE = "E2E Reviewer";

// Puts the e2e user back on the user role and removes the test role, whatever a failed run left.
async function cleanUp(request: APIRequestContext, userId: number) {
  const roles: { id: number; name: string }[] = (await (await request.get(`${API}/admin/roles?limit=100`)).json()).data;
  const userRole = roles.find((r) => r.name === "user")!;
  await send(request, "PUT", `/admin/users/${userId}/role`, { role_id: userRole.id });
  const left = roles.find((r) => r.name === ROLE);
  if (left) await send(request, "DELETE", `/admin/roles/${left.id}`);
}

test.describe("as admin", () => {
  test.use({ storageState: authFile("admin") });

  test("creates a role, gives it to a user, cannot delete it while held, then deletes it; each step is in the Logs", async ({
    page,
    request,
  }) => {
    const [user] = await (await request.get(`${API}/users?limit=100&email=${ACCOUNTS.user.email}`)).json();
    await cleanUp(request, user.id);
    try {
      await page.goto("/admin?tab=roles");
      const roles = page.getByRole("region", { name: "Roles" });
      await roles.getByRole("link", { name: "New role" }).click();
      await page.getByLabel("Name").fill(ROLE);
      await page.getByRole("checkbox", { name: /reports:read_any/ }).check();
      await page.getByRole("button", { name: "Create role" }).click();
      await expect(page.getByText(`Created role ${ROLE}`)).toBeVisible();
      const row = roles.getByRole("row").filter({ hasText: ROLE });
      await expect(row.getByRole("cell").nth(3)).toHaveText("1");

      await page.goto(`/admin/users/${user.id}/edit`);
      const section = page.getByRole("group", { name: "Role" });
      await section.getByLabel("Role").selectOption({ label: ROLE });
      await section.getByRole("button", { name: "Update role" }).click();
      await expect(page.getByText(`${ACCOUNTS.user.name} now has the ${ROLE} role`)).toBeVisible();

      await page.goto("/admin?tab=roles");
      await row.getByRole("button", { name: `Show the 1 user with role ${ROLE}` }).click();
      await expect(page.getByRole("alertdialog")).toContainText(ACCOUNTS.user.email);
      await page.getByRole("alertdialog").getByRole("button", { name: "Close" }).click();

      // Deleting a role in use moves its users to the chosen role first.
      await row.getByRole("button", { name: `Delete ${ROLE}` }).click();
      const dialog = page.getByRole("alertdialog");
      await expect(dialog).toContainText("1 user holds this role");
      await expect(dialog.getByRole("button", { name: "Move users and delete" })).toBeDisabled();
      await dialog.getByLabel("Move 1 user to").selectOption({ label: "user" });
      await dialog.getByRole("button", { name: "Move users and delete" }).click();
      await expect(page.getByText(`Moved 1 user to user and deleted role ${ROLE}`)).toBeVisible();
      await expect(row).toHaveCount(0);

      await page.goto("/admin?tab=logs");
      const logs = page.getByRole("region", { name: "Logs" });
      await expect(logs.getByText(/total logs$/)).toBeVisible();
      for (const [label, target] of [
        ["Created role", ROLE],
        ["Deleted role", ROLE],
        ["Changed user role", ACCOUNTS.user.email],
      ] as const) {
        await logs.getByLabel("Action").selectOption({ label });
        await expect(logs.getByRole("row").filter({ hasText: target }).first()).toBeVisible();
      }
    } finally {
      await cleanUp(request, user.id);
    }
  });
});

test.describe("as a plain user", () => {
  test.use({ storageState: authFile("user") });

  test("the API refuses role writes", async ({ request }) => {
    expect((await send(request, "POST", "/admin/roles", { name: "Nope", permission_ids: [] })).status()).toBe(403);
  });
});
