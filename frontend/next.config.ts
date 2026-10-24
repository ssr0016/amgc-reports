import type { NextConfig } from "next";

// Where /api/v1 is proxied to. The browser only ever talks to this site, so the
// API's cookies are first-party (SameSite Lax/Strict work) and CORS is not involved.
// Override per environment; the default is the production API.
export const API_ORIGIN = process.env.API_ORIGIN ?? "https://pastoral-reports-api.onrender.com";

const nextConfig: NextConfig = {
  /* config options here */
  // The e2e run builds into its own directory, so it can run while `next dev` holds .next.
  distDir: process.env.NEXT_DIST_DIR ?? ".next",
  cacheComponents: true,
  partialPrefetching: true,
  images: {
    remotePatterns: [
      {
        protocol: "https",
        hostname: "pub-34d82063b0224b23bf6873227edb3e55.r2.dev",
        port: "",
        pathname: "/**",
      },
    ],
  },
  // The home page is the reports list. Temporary (307) so browsers do not cache it.
  async redirects() {
    return [{ source: "/", destination: "/reports", permanent: false }];
  },
  // Same-origin API: set NEXT_PUBLIC_API_URL=/api/v1 and the browser's calls land here.
  // Local dev and e2e keep an absolute NEXT_PUBLIC_API_URL and never use this.
  async rewrites() {
    return [{ source: "/api/v1/:path*", destination: `${API_ORIGIN}/api/v1/:path*` }];
  },
  turbopack: {
    rules: {
      "*.css": {
        loaders: ["@tailwindcss/turbopack"],
        as: "*.css",
      },
    },
  },
};

export default nextConfig;
