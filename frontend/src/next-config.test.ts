import { describe, expect, it } from "vitest";
import nextConfig, { API_ORIGIN } from "../next.config";

describe("next.config rewrites", () => {
  it("proxies /api/v1 to the API origin, keeping the path", async () => {
    const rules = await nextConfig.rewrites!();
    expect(rules).toEqual([{ source: "/api/v1/:path*", destination: `${API_ORIGIN}/api/v1/:path*` }]);
  });

  it("defaults to the production API over https", () => {
    expect(API_ORIGIN).toMatch(/^https:\/\/[^/]+$/); // an origin: no path, no trailing slash
  });

  it("does not shadow the app's own routes", async () => {
    const rules = (await nextConfig.rewrites!()) as { source: string }[];
    expect(rules.every((r) => r.source.startsWith("/api/v1/"))).toBe(true);
  });
});
