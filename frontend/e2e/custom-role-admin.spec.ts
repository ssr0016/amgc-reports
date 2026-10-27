import AxeBuilder from "@axe-core/playwright";
import type { APIRequestContext } from "@playwright/test";
import { ACCOUNTS, API, authFile, clientIp, expect, previousPeriod, send, test } from "./support";

// 37b: a custom role opens /admin and sees only the admin links its permissions allow.
const ROLE = "E2E Panel Reviewer";
const PERMS = ["reports:read", "reports:read_any", "reports:approve", "tracker:read"];
const WORKER = ACCOUNTS.user.name; // the API sets worker to the owner's name

type Named = { id: number; name: string };

// Puts the e2e editor back on the editor role and removes the test role, whatever a failed run left.
async function cleanUp(admin: APIRequestContext, editorId: number) {
  const roles: Named[] = (await (await admin.get(`${API}/admin/roles?limit=100`)).json()).data;
  await send(admin, "PUT", `/admin/users/${editorId}/role`, { role_id: roles.find((r) => r.name === "editor")!.id });
  const left = roles.find((r) => r.name === ROLE);
  if (left) await send(admin, "DELETE", `/admin/roles/${left.id}`);
}

test.describe("as a custom role", () => {
  test.use({ storageState: authFile("editor") });

  test("sees only its admin links, approves a report, and cannot open Roles", async ({ page, playwright }) => {
    const admin = await playwright.request.newContext({
      storageState: authFile("admin"),
      extraHTTPHeaders: { "X-Real-IP": clientIp("custom-role-admin") },
    });
    const worker = await playwright.request.newContext({
      storageState: authFile("user"),
      extraHTTPHeaders: { "X-Real-IP": clientIp("custom-role-user") },
    });
    const [editor] = await (await admin.get(`${API}/users?limit=100&email=${ACCOUNTS.editor.email}`)).json();
    await cleanUp(admin, editor.id);

    // A plain user may file only the previous month; another spec may have left a report there, so start clean.
    const period = previousPeriod();
    const old = await (await worker.get(`${API}/reports?month=${period.month}&year=${period.year}`)).json();
    for (const r of old.data) await send(worker, "DELETE", `/reports/${r.id}`);
    const filed = await send(worker, "POST", "/reports", period);
    expect(filed.status(), await filed.text()).toBe(201);
    const { id: reportId } = await filed.json();

    try {
      const perms: Named[] = (await (await admin.get(`${API}/admin/permissions?limit=100`)).json()).data;
      const created = await send(admin, "POST", "/admin/roles", {
        name: ROLE,
        permission_ids: perms.filter((p) => PERMS.includes(p.name)).map((p) => p.id),
      });
      expect(created.status(), await created.text()).toBe(201);
      const assigned = await send(admin, "PUT", `/admin/users/${editor.id}/role`, { role_id: (await created.json()).id });
      expect(assigned.status(), await assigned.text()).toBe(200);

      await page.goto("/reports");
      await page.getByRole("navigation", { name: "Main" }).getByRole("link", { name: "Admin All Reports", exact: true }).click();
      await expect(page.getByRole("navigation", { name: "Main" }).getByRole("link", { name: /^Admin / })).toHaveText(["All Reports", "Analytics", "Tracker"]);
      await expect(page.getByRole("region", { name: "Report statistics" })).toBeVisible();

      const reports = page.getByRole("region", { name: "Reports" });
      const row = reports.getByRole("row").filter({ hasText: WORKER });
      await expect(row.getByText("Pending")).toBeVisible();
      await expect(reports.getByRole("button", { name: "Download Bulk" })).toHaveCount(0);
      await expect(row.getByRole("button", { name: /^Excel / })).toHaveCount(0);

      // Scan the restricted panel before Approve, so the toast's entry animation is not judged mid-fade.
      const { violations } = await new AxeBuilder({ page })
        .withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa", "best-practice"])
        .exclude("nextjs-portal")
        .analyze();
      expect(violations.map((v) => `${v.id} (${v.impact}): ${v.nodes.map((n) => n.target.join(" ")).join(" | ")}`)).toEqual([]);

      await row.getByRole("button", { name: /^Approve / }).click();
      await expect(row.getByText("Approved")).toBeVisible();

      await page.goto("/admin?tab=roles");
      await expect(page.getByRole("link", { name: "Admin All Reports", exact: true })).toHaveAttribute("aria-current", "page");
      await expect(page.getByRole("link", { name: "Admin Roles", exact: true })).toHaveCount(0);

      // The server agrees: account and role management stay admin-only.
      expect((await page.request.get(`${API}/admin/roles`)).status()).toBe(403);
    } finally {
      await send(worker, "DELETE", `/reports/${reportId}`);
      await cleanUp(admin, editor.id);
      await worker.dispose();
      await admin.dispose();
    }
  });
});
