import { test as base, type APIRequestContext, type APIResponse, type Page } from "@playwright/test";
import { defaultPeriod, MONTHS } from "../src/lib/reports";

export const API = "http://localhost:8081/api/v1";

export const ACCOUNTS = {
  admin: { email: "e2e-admin@example.com", password: "E2eAdmin!2345", name: "E2E Admin" },
  editor: { email: "e2e-editor@example.com", password: "E2eEditor!2345", name: "E2E Editor", role: "editor" },
  user: {
    email: "e2e-user@example.com",
    password: "E2eUser!23456",
    name: "E2E User",
    role: "user",
    area_assignment: "Muntinlupa",
    church_name: "Visionaires Christian Church",
  },
} as const;
export type Role = keyof typeof ACCOUNTS;

export const authFile = (role: Role) => `e2e/.auth/${role}.json`;

// The server limits logins per client address (5 a minute, burst 3) and trusts
// CF-Connecting-IP from the loopback proxy range that start-backend.sh configures. Giving
// every test its own address keeps one test's logins from starving the next.
export function clientIp(seed: string): string {
  let h = 0;
  for (const ch of seed) h = (h * 31 + ch.charCodeAt(0)) >>> 0;
  // Unsigned shifts: with `>>` a hash above 2^31 gave negative octets, which is not an address, so the
  // server fell back to 127.0.0.1 and those tests shared one login bucket.
  return `10.${(h >>> 16) % 250}.${(h >>> 8) % 250}.${(h % 250) + 1}`;
}

// The browser must not send CF-Connecting-IP itself: a custom header on a cross-origin call
// forces a CORS preflight the API does not allow. So the header is added on the wire
// (route.continue happens after the browser's CORS checks), and only the API-only
// `request` fixture sets it as a plain context header.
export const test = base.extend<{ clientIp: string }>({
  clientIp: async ({}, use, testInfo) => use(clientIp(testInfo.testId)),
  context: async ({ context, clientIp }, use) => {
    await context.route(`${API}/**`, (route) =>
      route.continue({ headers: { ...route.request().headers(), "cf-connecting-ip": clientIp } }),
    );
    await use(context);
  },
  request: async ({ playwright, storageState, clientIp }, use) => {
    const ctx = await playwright.request.newContext({ storageState, extraHTTPHeaders: { "CF-Connecting-IP": clientIp } });
    await use(ctx);
    await ctx.dispose();
  },
});
export { expect } from "@playwright/test";

export async function openProfileMenu(page: Page) {
  await page.getByRole("button", { name: / · / }).click();
}

export async function signOut(page: Page) {
  await openProfileMenu(page);
  await page.getByRole("menuitem", { name: "Sign Out" }).click();
}

// A write through the API, with the CSRF token the server wants.
export async function send(
  request: APIRequestContext,
  method: "POST" | "PUT" | "PATCH" | "DELETE",
  path: string,
  data?: unknown,
): Promise<APIResponse> {
  const token = (await (await request.get(`${API}/auth/csrf`)).json()).csrf_token;
  return request.fetch(`${API}${path}`, { method, data, headers: { "X-CSRF-Token": token } });
}

export const previousPeriod = () => defaultPeriod();

// The period `back` months before the previous month, for a report that is late.
export function earlierPeriod(back: number) {
  const { month, year } = defaultPeriod();
  const index = MONTHS.indexOf(month) - back;
  return { month: MONTHS[(index + 12) % 12], year: year + Math.floor(index / 12) };
}
