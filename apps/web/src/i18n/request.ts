import { getRequestConfig } from 'next-intl/server'
import { headers } from 'next/headers'
import { defaultLocale, fallbackChain, isSupported } from './registry'

// Stable list of namespaces. Adding a new namespace = append one entry here
// + drop the JSON in every messages/{locale}/ directory.
const NAMESPACES = [
  'nav',
  'tools',
  'auth',
  'home',
  'leaderboard',
  'nodes',
  'pricing',
  'errors',
  'common',
  'monitors',
  'alerts',
  'settings',
  'billing',
  'dashboard',
  'status',
  'admin',
] as const

type Namespace = (typeof NAMESPACES)[number]

// Mirror next-intl's `AbstractIntlMessages` (a recursive map of strings) so the
// payload typechecks at `getRequestConfig` and `NextIntlClientProvider` call sites.
type Messages = { [id: string]: Messages | string }

async function loadNamespace(locale: string, ns: Namespace): Promise<Messages> {
  for (const loc of fallbackChain(locale)) {
    try {
      const mod = await import(`./messages/${loc}/${ns}.json`)
      return mod.default as Messages
    } catch {
      // Try next locale in the fallback chain — missing namespace at this
      // locale is expected (e.g. admin in non-default locales).
    }
  }
  return {}
}

export async function loadMessages(locale: string): Promise<Messages> {
  const resolved = isSupported(locale) ? locale : defaultLocale
  const entries = await Promise.all(
    NAMESPACES.map(
      async (ns) => [ns, await loadNamespace(resolved, ns)] as const,
    ),
  )
  return Object.fromEntries(entries) as Messages
}

export default getRequestConfig(async ({ requestLocale }) => {
  const headersList = await headers()
  const headerLocale = headersList.get('x-locale') ?? ''
  const rawLocale = (await requestLocale) ?? headerLocale
  const locale = isSupported(rawLocale) ? rawLocale : defaultLocale

  return {
    locale,
    messages: await loadMessages(locale),
  }
})
