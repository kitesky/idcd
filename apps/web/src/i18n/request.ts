import { getRequestConfig } from 'next-intl/server'
import { headers } from 'next/headers'
import { defaultLocale, fallbackChain, isSupported } from './registry'
import { onError, getMessageFallback } from './error-handlers'

// Stable list of namespaces — must mirror messages/cn/*.json 1:1. Adding a new
// namespace = append one entry here + drop the JSON in every messages/{locale}/
// directory + add to types.d.ts. Missing files at a given locale are tolerated
// by loadNamespace's fallback chain (admin uses this in non-cn locales).
const NAMESPACES = [
  'about',
  'admin',
  'alerts',
  'auth',
  'billing',
  'common',
  'dashboard',
  'docs',
  'errors',
  'home',
  'incidents',
  'leaderboard',
  'monitors',
  'nav',
  'nodes',
  'pricing',
  'settings',
  'status',
  'tools',
  'userMenu',
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
    onError,
    getMessageFallback,
  }
})
