import { type NextRequest, NextResponse } from 'next/server'
import {
  defaultLocale,
  isSupported,
  localeCodes,
  negotiate,
  type Locale,
} from './i18n/registry'

// Non-default locales are the only ones that carry a URL prefix.
// Computed once at module load; updates require a rebuild (registry is static).
const PREFIXED_LOCALES = localeCodes.filter((c) => c !== defaultLocale)
const PREFIXED_LOCALES_SET = new Set(PREFIXED_LOCALES)

// Cookie names — keep reading the legacy `locale` cookie so existing sessions
// don't get bounced back to the default locale on first request after deploy.
const COOKIE_NAME = 'idcd_locale'
const LEGACY_COOKIE_NAME = 'locale'

/**
 * Match `/{code}/...` (or exactly `/{code}`) where `{code}` is a non-default
 * locale code from the registry.
 *
 * Returns the matched code + rest of the path (without the prefix), or null
 * when no prefix is present.
 */
function matchLocalePrefix(
  pathname: string,
): { code: Locale; rest: string } | null {
  for (const code of PREFIXED_LOCALES) {
    if (pathname === `/${code}`) return { code, rest: '/' }
    if (pathname.startsWith(`/${code}/`)) {
      return { code, rest: pathname.slice(code.length + 1) }
    }
  }
  return null
}

function readCookieLocale(request: NextRequest): Locale | null {
  const fresh = request.cookies.get(COOKIE_NAME)?.value
  if (fresh && isSupported(fresh)) return fresh
  const legacy = request.cookies.get(LEGACY_COOKIE_NAME)?.value
  if (legacy && isSupported(legacy)) return legacy
  return null
}

/**
 * Resolve the effective locale for a request.
 *
 * Order (highest priority first):
 *   1. URL prefix (`/{code}/...`)
 *   2. Cookie (`idcd_locale`, falling back to the legacy `locale` name)
 *   3. Accept-Language negotiation
 *   4. Registry default
 *
 * For authenticated areas (`/app`, `/auth`, `/admin`) we deliberately skip
 * Accept-Language because those flows persist locale via cookie + user.locale.
 */
function detectLocale(request: NextRequest, prefix: { code: Locale } | null): Locale {
  if (prefix) return prefix.code

  const cookie = readCookieLocale(request)
  if (cookie) return cookie

  const { pathname } = request.nextUrl
  const isAuthArea =
    pathname.startsWith('/app') ||
    pathname.startsWith('/auth') ||
    pathname.startsWith('/admin')
  if (isAuthArea) return defaultLocale

  return negotiate(request.headers.get('accept-language'))
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
  nonce?: string,
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
    'camera=(), microphone=(), geolocation=(), payment=(), interest-cohort=(), browsing-topics=()',
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

  // ── Normal request: detect locale, optionally rewrite prefix, inject headers
  const { pathname } = request.nextUrl
  const prefixMatch = matchLocalePrefix(pathname)
  const locale = detectLocale(request, prefixMatch)
  const nonce = crypto.randomUUID().replace(/-/g, '')

  const requestHeaders = new Headers({
    ...Object.fromEntries(request.headers),
    'x-nonce': nonce,
    'x-locale': locale,
  })

  // When the URL carries a non-default locale prefix, rewrite to the un-prefixed
  // path so the same page components render — the locale header drives
  // next-intl message loading inside the (shared) component tree.
  if (prefixMatch && PREFIXED_LOCALES_SET.has(prefixMatch.code)) {
    const rewriteUrl = request.nextUrl.clone()
    rewriteUrl.pathname = prefixMatch.rest
    const response = NextResponse.rewrite(rewriteUrl, {
      request: { headers: requestHeaders },
    })
    response.headers.set('x-locale', locale)
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
