import { describe, expect, it } from "vitest";

// A client-rendered app announces a route change from the document title (WCAG 2.4.2),
// so every page names itself; the root layout adds the " · AMGCReports" suffix.
describe("page titles", () => {
  it.each([
    ["./(app)/reports/page", "Reports"],
    ["./(app)/reports/new/page", "New report"],
    ["./(app)/reports/[id]/edit/page", "Edit report"],
    ["./(app)/admin/users/page", "Users"],
    ["./(app)/admin/users/new/page", "New user"],
    ["./(app)/admin/users/[id]/edit/page", "Edit user"],
    ["./login/layout", "Sign in"],
  ])("%s is %s", async (path, title) => {
    const mod = await import(/* @vite-ignore */ path);
    expect(mod.metadata?.title).toBe(title);
  });
});
