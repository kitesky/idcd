import type { Metadata } from 'next'
import { locales, defaultLocale } from '@/i18n/registry'

const SITE_ORIGIN = 'https://idcd.com'

/**
 * Build a canonical URL for the given path under the given locale.
 *
 * - `path` must start with `/` (e.g. `/about`).
 * - The default locale lives at the unprefixed origin (`/about`); every other
 *   locale lives under `/<code>` (e.g. `/en/about`).
 */
export function localizedUrl(path: string, locale: string): string {
  const safePath = path.startsWith('/') ? path : `/${path}`
  if (locale === defaultLocale) {
    return `${SITE_ORIGIN}${safePath}`
  }
  return `${SITE_ORIGIN}/${locale}${safePath === '/' ? '' : safePath}`
}

/**
 * Build a Next.js `Metadata.alternates` block that emits canonical + per-locale
 * hreflang `<link>` tags. Defaults `x-default` to the default-locale URL.
 *
 * @param path Path component without locale prefix (e.g. `/pricing`).
 * @param currentLocale Locale of the page being rendered (drives canonical).
 */
export function generateAlternates(
  path: string,
  currentLocale: string = defaultLocale,
): NonNullable<Metadata['alternates']> {
  const languages: Record<string, string> = {}
  for (const loc of locales) {
    languages[loc.bcp47] = localizedUrl(path, loc.code)
  }
  languages['x-default'] = localizedUrl(path, defaultLocale)

  return {
    canonical: localizedUrl(path, currentLocale),
    languages,
  }
}

/**
 * Convenience helper: produce localized metadata for a page that defines its
 * `title` / `description` via translations.
 */
export interface LocalizedMetadataInput {
  path: string
  locale?: string
  title?: string
  description?: string
  keywords?: string[]
  ogTitle?: string
  ogDescription?: string
  noindex?: boolean
}

export function buildLocalizedMetadata({
  path,
  locale = defaultLocale,
  title,
  description,
  keywords,
  ogTitle,
  ogDescription,
  noindex,
}: LocalizedMetadataInput): Metadata {
  const meta: Metadata = {
    alternates: generateAlternates(path, locale),
  }
  if (title) meta.title = title
  if (description) meta.description = description
  if (keywords && keywords.length > 0) meta.keywords = keywords
  if (noindex) meta.robots = { index: false, follow: false }
  if (title || description) {
    meta.openGraph = {
      title: ogTitle ?? title,
      description: ogDescription ?? description,
      url: localizedUrl(path, locale),
      siteName: 'idcd',
      type: 'website',
    }
  }
  return meta
}
