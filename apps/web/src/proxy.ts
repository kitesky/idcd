import { NextResponse } from "next/server"
import type { NextRequest } from "next/server"

export function proxy(request: NextRequest) {
  const nonce = crypto.randomUUID().replace(/-/g, "")
  const isDev = process.env.NODE_ENV === "development"

  const csp = [
    `default-src 'self'`,
    // Next.js requires 'self' plus the nonce for its injected runtime scripts.
    `script-src 'self' 'nonce-${nonce}'${isDev ? " 'unsafe-eval'" : ""}`,
    // Tailwind inline styles are unavoidable without a full CSS extraction step.
    `style-src 'self' 'unsafe-inline'`,
    `img-src 'self' data: https:`,
    `font-src 'self' data:`,
    `connect-src 'self' https://api.idcd.com${isDev ? " http://localhost:8080 ws://localhost:3000" : ""}`,
    `frame-ancestors 'none'`,
  ].join("; ")

  const response = NextResponse.next({
    request: {
      headers: new Headers({
        ...Object.fromEntries(request.headers),
        "x-nonce": nonce,
      }),
    },
  })

  response.headers.set("Content-Security-Policy", csp)
  response.headers.set("X-Frame-Options", "DENY")
  response.headers.set(
    "Permissions-Policy",
    "camera=(), microphone=(), geolocation=(), payment=(), interest-cohort=(), browsing-topics=()"
  )

  return response
}

export const config = {
  matcher: [
    // Apply to all routes except static files and Next.js internals.
    "/((?!_next/static|_next/image|favicon.ico).*)",
  ],
}
