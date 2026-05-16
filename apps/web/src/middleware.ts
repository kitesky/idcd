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

export function middleware(request: NextRequest): NextResponse {
  const locale = detectLocale(request)
  const response = NextResponse.next()
  response.headers.set('x-locale', locale)
  return response
}

export const config = {
  matcher: ['/((?!api|_next/static|_next/image|favicon.ico|.*\\..*).*)'],
}
