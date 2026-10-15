import { expect, test as setup } from "@playwright/test";
import { ACCOUNTS, API, authFile, clientIp, type Role } from "./support";

// Signs in as the seeded admin, creates the editor and user accounts through the
// admin API, then signs each role in once and saves its session for the specs.
setup("create accounts and save one session per role", async ({ playwright }) => {
  async function signIn(role: Role) {
    const ctx = await playwright.request.newContext({ extraHTTPHeaders: { "X-Real-IP": clientIp(`setup-${role}`) } });
    const csrf = async () => (await (await ctx.get(`${API}/auth/csrf`)).json()).csrf_token;
    const login = await ctx.post(`${API}/auth/login`, {
      data: { email: ACCOUNTS[role].email, password: ACCOUNTS[role].password },
      headers: { "X-CSRF-Token": await csrf() },
    });
    expect(login.status(), `login as ${role}`).toBe(200);
    return { ctx, csrf };
  }

  const admin = await signIn("admin");
  const roles: { id: number; name: string }[] = (await (await admin.ctx.get(`${API}/admin/roles?limit=100`)).json()).data;
  for (const role of ["editor", "user"] as const) {
    const { password, email, name, ...rest } = ACCOUNTS[role];
    const roleId = roles.find((r) => r.name === rest.role)?.id;
    expect(roleId, `seeded role ${rest.role}`).toBeDefined();
    const res = await admin.ctx.post(`${API}/admin/users`, {
      data: {
        email, name, password, role_id: roleId,
        area_assignment: "area_assignment" in rest ? rest.area_assignment : "",
        church_name: "church_name" in rest ? rest.church_name : "",
      },
      headers: { "X-CSRF-Token": await admin.csrf() },
    });
    expect(res.status(), `create ${role}: ${await res.text()}`).toBe(201);
  }
  await admin.ctx.storageState({ path: authFile("admin") });
  await admin.ctx.dispose();

  for (const role of ["editor", "user"] as const) {
    const { ctx } = await signIn(role);
    await ctx.storageState({ path: authFile(role) });
    await ctx.dispose();
  }
});
