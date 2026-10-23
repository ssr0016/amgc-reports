import { NextRequest } from "next/server";
import { describe, expect, it } from "vitest";
import { proxy } from "./proxy";

const visit = (path: string, cookie?: string) =>
  proxy(new NextRequest(`http://app.test${path}`, { headers: cookie ? { cookie } : {} }));

describe("proxy", () => {
  it("lets a visitor with the signed_in hint through", () => {
    expect(visit("/admin", "signed_in=1").headers.get("location")).toBeNull();
  });

  it("sends anyone else to /login, keeping the page as ?next= except for home", () => {
    expect(visit("/admin/users?tab=x").headers.get("location")).toBe(
      "http://app.test/login?next=%2Fadmin%2Fusers%3Ftab%3Dx",
    );
    expect(visit("/reports").headers.get("location")).toBe("http://app.test/login");
  });
});
