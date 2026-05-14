import type { Metadata } from 'next'
import { notFound } from 'next/navigation'
import { EN_TOOLS_META, getEnToolMeta } from '@/i18n/en-tools-meta'
import ToolRenderer from '@/app/tools/[slug]/tool-renderer'

type Props = {
  params: Promise<{ slug: string }>
}

export async function generateStaticParams() {
  return EN_TOOLS_META.map(tool => ({ slug: tool.slug }))
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { slug } = await params
  const tool = getEnToolMeta(slug)

  if (!tool) {
    return {
      title: `${slug} | idcd Tools`,
      description: `idcd online network tool: ${slug}`,
    }
  }

  return {
    title: tool.title,
    description: tool.description,
    alternates: {
      canonical: `https://idcd.com/en/tools/${slug}`,
      languages: {
        'zh': `https://idcd.com/tools/${slug}`,
        'en': `https://idcd.com/en/tools/${slug}`,
      },
    },
    openGraph: {
      title: tool.title,
      description: tool.description,
      url: `https://idcd.com/en/tools/${slug}`,
      type: 'website',
      siteName: 'idcd',
    },
  }
}

export default async function EnToolPage({ params }: Props) {
  const { slug } = await params
  const tool = getEnToolMeta(slug)

  if (!tool) {
    notFound()
  }

  const jsonLd = {
    '@context': 'https://schema.org',
    '@type': 'WebApplication',
    name: tool.schemaName,
    description: tool.description,
    url: `https://idcd.com/en/tools/${slug}`,
    applicationCategory: 'UtilityApplication',
    operatingSystem: 'Web',
    inLanguage: 'en',
    offers: {
      '@type': 'Offer',
      price: '0',
      priceCurrency: 'USD',
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
      <ToolRenderer slug={slug} />
    </>
  )
}
