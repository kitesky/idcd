import type { Metadata } from 'next'
import { Suspense } from 'react'
import { ALL_TOOLS, getToolBySlug } from '@/app/(public)/tools/tools-config'
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

  return {
    title: tool?.metaTitle ?? `${slug} | idcd 工具`,
    description: tool?.metaDescription ?? `idcd 在线工具：${slug}`,
    openGraph: {
      title: tool?.metaTitle ?? `${slug} | idcd`,
      description: tool?.metaDescription ?? '',
      type: 'website',
    },
  }
}

export default async function ToolSlugPage({ params }: Props) {
  const { slug } = await params
  const tool = getToolBySlug(slug)

  const jsonLd = {
    '@context': 'https://schema.org',
    '@type': 'WebApplication',
    name: tool?.name ?? slug,
    description: tool?.metaDescription ?? '',
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
