import { headers, cookies } from 'next/headers'
import { defaultLocale, isSupported, type Locale } from './registry'

// Cookie naming: new canonical name is `idcd_locale`. Older sessions wrote
// `locale` — we still honor that to avoid forcing users back to the default.
const COOKIE_NAME = 'idcd_locale'
const LEGACY_COOKIE_NAME = 'locale'

export async function getLocale(): Promise<Locale> {
  const headersList = await headers()
  const h = headersList.get('x-locale') ?? ''
  return isSupported(h) ? h : defaultLocale
}

export async function getLocaleCookie(): Promise<Locale> {
  const cookieStore = await cookies()
  const fresh = cookieStore.get(COOKIE_NAME)?.value
  if (fresh && isSupported(fresh)) return fresh
  const legacy = cookieStore.get(LEGACY_COOKIE_NAME)?.value
  if (legacy && isSupported(legacy)) return legacy
  return defaultLocale
}

export { COOKIE_NAME as LOCALE_COOKIE_NAME, LEGACY_COOKIE_NAME as LEGACY_LOCALE_COOKIE_NAME }
