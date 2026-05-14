import { NextResponse } from "next/server"
import type { NextRequest } from "next/server"

/**
 * Middleware that handles custom domain routing for the status page app.
 *
 * Two cases:
 *  1. Standard subdomain: <slug>.status.idcd.com → serve /[slug] normally.
 *  2. Custom domain: e.g. status.example.com → look up the slug via the
 *     internal API and rewrite to /[slug]?customDomain=<host>.
 *
 * The `customDomain` search param is consumed by /[slug]/page.tsx so it can
 * fall back to an API call instead of reading from MOCK_STATUS_PAGES.
 */
export function middleware(request: NextRequest) {
  const host = request.headers.get("host") ?? ""
  // Strip port for local dev (e.g. localhost:3001)
  const hostname = host.split(":")[0]

  const isIdcdSubdomain =
    hostname === "status.idcd.com" ||
    hostname.endsWith(".status.idcd.com") ||
    hostname === "localhost" ||
    hostname === "127.0.0.1"

  if (!isIdcdSubdomain && hostname !== "") {
    // This is a custom domain request.
    // Attach the custom domain as a search param so the page component can
    // resolve the slug by calling the internal API.
    const url = request.nextUrl.clone()
    url.searchParams.set("customDomain", hostname)
    return NextResponse.rewrite(url)
  }

  return NextResponse.next()
}

export const config = {
  /**
   * Run middleware on all routes except Next.js internals and static assets.
   * The negative lookahead keeps _next/static, _next/image, and favicon.ico fast.
   */
  matcher: ["/((?!api|_next/static|_next/image|favicon.ico).*)"],
}
