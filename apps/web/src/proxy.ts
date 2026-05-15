import { NextResponse } from "next/server"
import type { NextRequest } from "next/server"

/**
 * Combined proxy: multi-domain routing + CSP/security headers.
 *
 * Domain routing (from former middleware.ts):
 *   - <slug>.status.idcd.com  → rewrite to /status/<slug>
 *   - Custom domain           → rewrite to /status/__resolve?customDomain=<host>
 *
 * Security headers applied to all responses.
 */
export function proxy(request: NextRequest) {
  const host = request.headers.get("host") ?? ""
  const hostname = host.split(":")[0]!
  const isDev = process.env.NODE_ENV === "development"

  // ── Status subdomain: <slug>.status.idcd.com ──────────────────────────────
  if (hostname.endsWith(".status.idcd.com") && hostname !== "status.idcd.com") {
    const slug = hostname.slice(0, -".status.idcd.com".length)
    const url = request.nextUrl.clone()
    url.pathname = `/status/${encodeURIComponent(slug)}`
    return withSecurityHeaders(NextResponse.rewrite(url), isDev)
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
    return withSecurityHeaders(NextResponse.rewrite(url), isDev)
  }

  // ── All other requests: pass through with security headers ─────────────────
  const nonce = crypto.randomUUID().replace(/-/g, "")
  const response = NextResponse.next({
    request: {
      headers: new Headers({
        ...Object.fromEntries(request.headers),
        "x-nonce": nonce,
      }),
    },
  })
  return withSecurityHeaders(response, isDev, nonce)
}

function withSecurityHeaders(
  response: NextResponse,
  isDev: boolean,
  nonce?: string
): NextResponse {
  const csp = [
    `default-src 'self'`,
    `script-src 'self'${nonce ? ` 'nonce-${nonce}'` : ""}${isDev ? " 'unsafe-eval'" : ""}`,
    `style-src 'self' 'unsafe-inline'`,
    `img-src 'self' data: https:`,
    `font-src 'self' data:`,
    `connect-src 'self' https://api.idcd.com${isDev ? " http://localhost:8080 ws://localhost:3000" : ""}`,
    `frame-ancestors 'none'`,
  ].join("; ")

  response.headers.set("Content-Security-Policy", csp)
  response.headers.set("X-Frame-Options", "DENY")
  response.headers.set(
    "Permissions-Policy",
    "camera=(), microphone=(), geolocation=(), payment=(), interest-cohort=(), browsing-topics=()"
  )
  return response
}

export const config = {
  matcher: ["/((?!_next/static|_next/image|favicon.ico).*)"],
}
