import { type NextRequest, NextResponse } from 'next/server'
import { type Locale, defaultLocale } from './i18n/routing'

function detectLocale(request: NextRequest): Locale {
  const { pathname } = request.nextUrl

  // URL prefix wins for public pages under /en
  if (pathname.startsWith('/en')) return 'en'

  // For authenticated and admin pages: read cookie
  if (
    pathname.startsWith('/app') ||
    pathname.startsWith('/auth') ||
    pathname.startsWith('/admin')
  ) {
    const cookie = request.cookies.get('locale')?.value
    if (cookie === 'en') return 'en'
    return defaultLocale
  }

  return defaultLocale
}

// NEXT_PUBLIC_API_URL is baked at build time; resolve once and cache. Malformed
// values fall back to the prod origin (fail-closed) — never echo the raw string
// into CSP, that would poison the entire connect-src directive.
const API_ORIGIN: string = (() => {
  const raw = process.env.NEXT_PUBLIC_API_URL ?? ''
  try {
    return raw ? new URL(raw).origin : 'https://api.idcd.com'
  } catch {
    return 'https://api.idcd.com'
  }
})()

function withSecurityHeaders(
  response: NextResponse,
  isDev: boolean,
  nonce?: string
): NextResponse {
  const csp = [
    `default-src 'self'`,
    `script-src 'self'${nonce ? ` 'nonce-${nonce}'` : ''}${isDev ? " 'unsafe-eval'" : ''}`,
    `style-src 'self' 'unsafe-inline'`,
    `img-src 'self' data: https:`,
    `font-src 'self' data:`,
    `connect-src 'self' ${API_ORIGIN}${isDev ? ' http://localhost:8080 ws://localhost:3000' : ''}`,
    `frame-ancestors 'none'`,
  ].join('; ')

  response.headers.set('Content-Security-Policy', csp)
  response.headers.set('X-Frame-Options', 'DENY')
  response.headers.set(
    'Permissions-Policy',
    'camera=(), microphone=(), geolocation=(), payment=(), interest-cohort=(), browsing-topics=()'
  )
  return response
}

export function proxy(request: NextRequest): NextResponse {
  const host = request.headers.get('host') ?? ''
  const hostname = host.split(':')[0]!
  const isDev = process.env.NODE_ENV === 'development'

  // ── Status subdomain: <slug>.status.idcd.com ─────────────────────────────
  if (hostname.endsWith('.status.idcd.com') && hostname !== 'status.idcd.com') {
    const slug = hostname.slice(0, -'.status.idcd.com'.length)
    const url = request.nextUrl.clone()
    url.pathname = `/status/${encodeURIComponent(slug)}`
    return withSecurityHeaders(NextResponse.rewrite(url), isDev)
  }

  // ── Custom domain → resolve via API in page component ────────────────────
  const isIdcdOrLocal =
    hostname === 'idcd.com' ||
    hostname.endsWith('.idcd.com') ||
    hostname === 'localhost' ||
    hostname === '127.0.0.1' ||
    hostname === ''

  if (!isIdcdOrLocal) {
    const url = request.nextUrl.clone()
    url.pathname = '/status/__resolve'
    url.searchParams.set('customDomain', hostname)
    return withSecurityHeaders(NextResponse.rewrite(url), isDev)
  }

  // ── Normal request: add locale + nonce + security headers ─────────────────
  const locale = detectLocale(request)
  const nonce = crypto.randomUUID().replace(/-/g, '')

  const requestHeaders = new Headers({
    ...Object.fromEntries(request.headers),
    'x-nonce': nonce,
    'x-locale': locale,
  })

  // Rewrite /en/<path> → /<path> for pages that don't have an explicit /en/* file.
  // Existing file-based routes (/en and /en/tools/[slug]) are excluded so they
  // continue to render their own page components.
  const { pathname } = request.nextUrl
  const isFileBasedEnRoute =
    pathname === '/en' || pathname.startsWith('/en/tools/')

  if (locale === 'en' && pathname.startsWith('/en/') && !isFileBasedEnRoute) {
    const rewriteUrl = request.nextUrl.clone()
    rewriteUrl.pathname = pathname.slice(3) // strip /en prefix
    const response = NextResponse.rewrite(rewriteUrl, {
      request: { headers: requestHeaders },
    })
    response.headers.set('x-locale', 'en')
    return withSecurityHeaders(response, isDev, nonce)
  }

  const response = NextResponse.next({
    request: { headers: requestHeaders },
  })

  response.headers.set('x-locale', locale)
  return withSecurityHeaders(response, isDev, nonce)
}

export const config = {
  matcher: ['/((?!_next/static|_next/image|favicon.ico).*)'],
}
