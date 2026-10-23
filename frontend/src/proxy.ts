import { NextResponse, type NextRequest } from "next/server";
import { HOME, SIGNED_IN_COOKIE } from "@/lib/routes";

// Sends a signed-out visitor to /login before the page renders, instead of the client guard's
// "Loading…". The cookie is only a hint set by the session; the API still checks every request,
// and the (app) layout still redirects when /auth/me says otherwise.
export function proxy(request: NextRequest) {
  if (request.cookies.has(SIGNED_IN_COOKIE)) return NextResponse.next();
  const { pathname, search } = request.nextUrl;
  const path = pathname + search;
  const url = new URL("/login", request.url);
  // Landing on the home page after signing in is the default, so no ?next= for it.
  if (path !== HOME) url.searchParams.set("next", path);
  return NextResponse.redirect(url);
}

export const config = {
  matcher: ["/reports/:path*", "/admin/:path*", "/account/:path*"],
};
