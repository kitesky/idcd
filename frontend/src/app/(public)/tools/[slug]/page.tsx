import type { Metadata } from 'next'
import { Suspense } from 'react'
import { getT } from '@/i18n/getT'
import { getLocale } from '@/i18n/locale'
import { ALL_TOOLS, getToolBySlug } from '@/app/(public)/tools/tools-config'
import { generateAlternates, localizedUrl } from '@/lib/seo'
import { bcp47Of, currencyOf } from '@/i18n/registry'
import ToolRenderer from './tool-renderer'

type Props = {
  params: Promise<{ slug: string }>
}

export async function generateStaticParams() {
  return ALL_TOOLS.map(tool => ({ slug: tool.slug }))
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { slug } = await params
  const tool = getToolBySlug(slug)
  const locale = await getLocale()
  const path = `/tools/${slug}`

  const t = await getT('tools', locale)
  // `getT` returns the key itself when the lookup misses, so detect that to
  // decide whether we have a usable translation.
  const titleKey = `${slug}.meta.title`
  const descKey = `${slug}.meta.description`
  const rawTitle = t(titleKey)
  const rawDesc = t(descKey)
  const metaTitle = rawTitle !== titleKey ? rawTitle : (tool ? `${slug} | idcd` : `${slug} | idcd`)
  const metaDescription = rawDesc !== descKey ? rawDesc : ''

  return {
    title: metaTitle,
    description: metaDescription,
    alternates: generateAlternates(path, locale),
    openGraph: {
      title: metaTitle,
      description: metaDescription,
      url: localizedUrl(path, locale),
      siteName: 'idcd',
      type: 'website',
    },
  }
}

export default async function ToolSlugPage({ params }: Props) {
  const { slug } = await params
  const locale = await getLocale()
  const t = await getT('tools', locale)

  const toolName = t(`${slug}.title`)
  const toolMetaDescription = t(`${slug}.meta.description`)

  const jsonLd = {
    '@context': 'https://schema.org',
    '@type': 'WebApplication',
    name: toolName === `${slug}.title` ? slug : toolName,
    description: toolMetaDescription === `${slug}.meta.description` ? '' : toolMetaDescription,
    url: localizedUrl(`/tools/${slug}`, locale),
    applicationCategory: 'UtilityApplication',
    operatingSystem: 'Web',
    inLanguage: bcp47Of(locale),
    offers: {
      '@type': 'Offer',
      price: '0',
      priceCurrency: currencyOf(locale),
    },
    publisher: {
      '@type': 'Organization',
      name: 'idcd',
      url: 'https://idcd.com',
    },
  }

  return (
    <>
      <script
        type="application/ld+json"
        // XSS: escape `<` so a translated value containing `</script>` can't break out
        dangerouslySetInnerHTML={{ __html: JSON.stringify(jsonLd).replace(/</g, '\\u003c') }}
      />
      <Suspense fallback={null}>
        <ToolRenderer slug={slug} />
      </Suspense>
    </>
  )
}
