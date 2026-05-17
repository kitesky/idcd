import { MetadataRoute } from 'next'
import { ALL_TOOLS } from '@/app/(public)/tools/tools-config'
import { locales, defaultLocale, bcp47Of } from '@/i18n/registry'

const SITE_ORIGIN = 'https://idcd.com'

interface PageEntry {
  url: string
  priority: number
  changeFrequency: 'always' | 'hourly' | 'daily' | 'weekly' | 'monthly' | 'yearly' | 'never'
}

function buildAlternates(path: string): Record<string, string> {
  const map: Record<string, string> = {}
  for (const loc of locales) {
    const target =
      loc.code === defaultLocale
        ? `${SITE_ORIGIN}${path}`
        : `${SITE_ORIGIN}/${loc.code}${path === '/' ? '' : path}`
    map[bcp47Of(loc.code)] = target
  }
  return map
}

function localizedUrl(path: string, locale: string): string {
  if (locale === defaultLocale) return `${SITE_ORIGIN}${path}`
  return `${SITE_ORIGIN}/${locale}${path === '/' ? '' : path}`
}

export default function sitemap(): MetadataRoute.Sitemap {
  const now = new Date()

  // Public pages — emit one entry per locale, each cross-linked via `alternates`.
  const mainPages: PageEntry[] = [
    { url: '/', priority: 1.0, changeFrequency: 'weekly' },
    { url: '/nodes', priority: 0.8, changeFrequency: 'weekly' },
    { url: '/about', priority: 0.8, changeFrequency: 'monthly' },
    { url: '/pricing', priority: 0.9, changeFrequency: 'weekly' },
    { url: '/agent', priority: 0.9, changeFrequency: 'weekly' },
    { url: '/monitors', priority: 0.9, changeFrequency: 'weekly' },
    { url: '/leaderboard', priority: 0.8, changeFrequency: 'weekly' },
    { url: '/transparency', priority: 0.7, changeFrequency: 'weekly' },
    { url: '/terms', priority: 0.8, changeFrequency: 'yearly' },
    { url: '/privacy', priority: 0.8, changeFrequency: 'yearly' },
    { url: '/aup', priority: 0.8, changeFrequency: 'yearly' },
  ]

  const toolPages: PageEntry[] = ALL_TOOLS.map((tool) => ({
    url: `/tools/${tool.slug}`,
    priority: 0.7,
    changeFrequency: 'monthly' as const,
  }))

  const entries: MetadataRoute.Sitemap = []
  for (const page of [...mainPages, ...toolPages]) {
    const alternates = { languages: buildAlternates(page.url) }
    for (const loc of locales) {
      entries.push({
        url: localizedUrl(page.url, loc.code),
        lastModified: now,
        changeFrequency: page.changeFrequency,
        priority: page.priority,
        alternates,
      })
    }
  }
  return entries
}
