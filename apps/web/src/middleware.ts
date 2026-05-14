import { NextResponse } from "next/server"
import type { NextRequest } from "next/server"

/**
 * Multi-domain routing middleware for the unified web app.
 *
 * Handles two status-page domain patterns:
 *   1. <slug>.status.idcd.com  → rewrite internally to /status/<slug>
 *   2. Custom domain (e.g. status.mycompany.com) → rewrite to /status/__resolve
 *      with ?customDomain=<host> so the page can look up the slug via API.
 *
 * All other requests (idcd.com, localhost, etc.) pass through unchanged.
 */
export function middleware(request: NextRequest) {
  const host = request.headers.get("host") ?? ""
  const hostname = host.split(":")[0]

  // ── Status subdomain: <slug>.status.idcd.com ──────────────────────────────
  if (hostname.endsWith(".status.idcd.com") && hostname !== "status.idcd.com") {
    const slug = hostname.slice(0, -".status.idcd.com".length)
    const url = request.nextUrl.clone()
    url.pathname = `/status/${encodeURIComponent(slug)}`
    return NextResponse.rewrite(url)
  }

  // ── Custom domain → resolve via API in the page component ─────────────────
  const isIdcdOrLocal =
    hostname === "idcd.com" ||
    hostname.endsWith(".idcd.com") ||
    hostname === "localhost" ||
    hostname === "127.0.0.1" ||
    hostname === ""

  if (!isIdcdOrLocal) {
    const url = request.nextUrl.clone()
    url.pathname = "/status/__resolve"
    url.searchParams.set("customDomain", hostname)
    return NextResponse.rewrite(url)
  }

  return NextResponse.next()
}

export const config = {
  matcher: ["/((?!api|_next/static|_next/image|favicon.ico).*)"],
}
