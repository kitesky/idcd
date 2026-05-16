import type { Metadata } from 'next'
import { Suspense } from 'react'
import { getTranslations } from 'next-intl/server'
import { ALL_TOOLS, getToolBySlug } from '@/app/(public)/tools/tools-config'
import { getLocale } from '@/i18n/locale'
import ToolRenderer from './tool-renderer'

type Props = {
  params: Promise<{ slug: string }>
}

export async function generateStaticParams() {
  return ALL_TOOLS.map(tool => ({ slug: tool.slug }))
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { slug } = await params
  const locale = await getLocale()
  const tool = getToolBySlug(slug)

  try {
    const t = await getTranslations({ locale, namespace: 'tools' })
    // t.has() is not available; use try/catch to handle missing keys gracefully
    const metaTitle = t(`${slug}.meta.title`)
    const metaDescription = t(`${slug}.meta.description`)
    return {
      title: metaTitle,
      description: metaDescription,
      openGraph: {
        title: metaTitle,
        description: metaDescription,
        type: 'website',
      },
    }
  } catch {
    // Fallback for slugs not yet in translation files
    return {
      title: tool?.name ? `${tool.name} | idcd` : `${slug} | idcd 工具`,
      description: tool?.description ?? `idcd 在线工具：${slug}`,
      openGraph: {
        title: tool?.name ? `${tool.name} | idcd` : `${slug} | idcd`,
        description: tool?.description ?? '',
        type: 'website',
      },
    }
  }
}

export default async function ToolSlugPage({ params }: Props) {
  const { slug } = await params
  const tool = getToolBySlug(slug)
  const locale = await getLocale()

  let toolName = tool?.name ?? slug
  let toolMetaDescription = tool?.description ?? ''

  try {
    const t = await getTranslations({ locale, namespace: 'tools' })
    toolName = t(`${slug}.title`)
    toolMetaDescription = t(`${slug}.meta.description`)
  } catch {
    // fallback to config values
  }

  const jsonLd = {
    '@context': 'https://schema.org',
    '@type': 'WebApplication',
    name: toolName,
    description: toolMetaDescription,
    url: `https://idcd.com/tools/${slug}`,
    applicationCategory: 'UtilityApplication',
    operatingSystem: 'Web',
    offers: {
      '@type': 'Offer',
      price: '0',
      priceCurrency: 'CNY',
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
        dangerouslySetInnerHTML={{ __html: JSON.stringify(jsonLd) }}
      />
      <Suspense fallback={null}>
        <ToolRenderer slug={slug} />
      </Suspense>
    </>
  )
}
