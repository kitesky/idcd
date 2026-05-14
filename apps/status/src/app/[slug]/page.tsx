import type { Metadata } from "next"
import { notFound } from "next/navigation"
import { MOCK_STATUS_PAGES } from "./mock-data"
import { StatusClient } from "./status-client"

interface Props {
  params: Promise<{ slug: string }>
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { slug } = await params
  const data = MOCK_STATUS_PAGES[slug]
  if (!data) return { title: "状态页未找到" }
  return {
    title: `${data.title} — 状态页`,
    description: `查看 ${data.title} 的实时服务可用性状态`,
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
