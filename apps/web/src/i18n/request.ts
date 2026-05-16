import { getRequestConfig } from 'next-intl/server'
import { headers } from 'next/headers'
import { defaultLocale, fallbackChain, isSupported } from './registry'

// Stable list of namespaces. Adding a new namespace = append one entry here
// + drop the JSON in every messages/{locale}/ directory. Missing files at a
// given locale are tolerated by loadNamespace's fallback chain.
const NAMESPACES = [
  'nav',
  'footer',
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
  'statusPages',
  'incidents',
  'userMenu',
  'admin',
  'docs',
  'enums',
  'validation',
  'about',
  'transparency',
  'legal',
] as const

type Namespace = (typeof NAMESPACES)[number]

// Mirror next-intl's `AbstractIntlMessages` (a recursive map of strings) so the
// payload typechecks at `getRequestConfig` and `NextIntlClientProvider` call sites.
type Messages = { [id: string]: Messages | string }

function deepMerge(base: Messages, overlay: Messages): Messages {
  const result: Messages = { ...base }
  for (const [key, value] of Object.entries(overlay)) {
    const baseVal = result[key]
    if (
      typeof value === 'object' &&
      value !== null &&
      typeof baseVal === 'object' &&
      baseVal !== null
    ) {
      result[key] = deepMerge(baseVal, value)
    } else {
      result[key] = value
    }
  }
  return result
}

async function loadNamespace(locale: string, ns: Namespace): Promise<Messages> {
  // Walk the fallback chain back-to-front and deep-merge so that a target
  // locale with partial coverage (e.g. admin in EN) inherits every missing
  // key from its fallback. Per-key fallback is required by I18N-PLAN.md D2.
  const chain = fallbackChain(locale)
  let merged: Messages = {}
  let hadAny = false
  for (let i = chain.length - 1; i >= 0; i--) {
    try {
      const mod = await import(`./messages/${chain[i]}/${ns}.json`)
      merged = deepMerge(merged, mod.default as Messages)
      hadAny = true
    } catch {
      // Missing namespace at this locale is tolerated — keep the merge going.
    }
  }
  return hadAny ? merged : {}
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
