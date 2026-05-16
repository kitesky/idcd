import { headers, cookies } from 'next/headers'
import { type Locale, defaultLocale, isValidLocale } from './routing'

export async function getLocale(): Promise<Locale> {
  const headersList = await headers()
  const h = headersList.get('x-locale') ?? ''
  return isValidLocale(h) ? h : defaultLocale
}

export async function getLocaleCookie(): Promise<Locale> {
  const cookieStore = await cookies()
  const val = cookieStore.get('locale')?.value ?? ''
  return isValidLocale(val) ? val : defaultLocale
}
