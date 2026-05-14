import type { Metadata } from "next"
import { notFound } from "next/navigation"
import { MOCK_STATUS_PAGES } from "./mock-data"
import { StatusClient } from "./status-client"

// Revalidate every 60 seconds so the status page reflects near-real-time data
// without blocking every request. Adjust downward once real data is wired in.
export const revalidate = 60

interface Props {
  params: Promise<{ slug: string }>
}

export async function generateStaticParams() {
  return Object.keys(MOCK_STATUS_PAGES).map((slug) => ({ slug }))
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { slug } = await params
  const data = MOCK_STATUS_PAGES[slug]
  if (!data) return { title: "状态页未找到" }
  return {
    title: `${data.title} — 状态页`,
    description: `查看 ${data.title} 的实时服务可用性状态`,
    openGraph: {
      title: `${data.title} — 状态页`,
      description: `查看 ${data.title} 的实时服务可用性状态`,
      type: "website",
    },
    twitter: {
      card: "summary",
      title: `${data.title} — 状态页`,
      description: `查看 ${data.title} 的实时服务可用性状态`,
    },
  }
}

export default async function StatusPage({ params }: Props) {
  const { slug } = await params
  const data = MOCK_STATUS_PAGES[slug]

  if (!data) {
    notFound()
  }

  return <StatusClient data={data} />
}
