import { cache } from "react"
import type { Metadata } from "next"
import { notFound } from "next/navigation"
import { StatusClient } from "./status-client"
import type { StatusPageData } from "./types"

export const revalidate = 60

interface Props {
  params: Promise<{ slug: string }>
  searchParams: Promise<{ customDomain?: string }>
}

const resolveSlugFromCustomDomain = cache(async (customDomain: string): Promise<string | null> => {
  const apiBase = process.env.INTERNAL_API_URL ?? "http://localhost:8080"
  try {
    const res = await fetch(`${apiBase}/internal/status-pages/by-domain?domain=${encodeURIComponent(customDomain)}`, { cache: "no-store" })
    if (!res.ok) return null
    const json = (await res.json()) as { data?: { slug?: string } }
    return json.data?.slug ?? null
  } catch { return null }
})

const fetchStatusPage = cache(async (slug: string): Promise<StatusPageData | null> => {
  const apiBase = process.env.INTERNAL_API_URL ?? "http://localhost:8080"
  try {
    const res = await fetch(`${apiBase}/v1/status-pages/${encodeURIComponent(slug)}/public`, {
      next: { revalidate: 60 },
    })
    if (!res.ok) return null
    const json = (await res.json()) as { data?: StatusPageData }
    return json.data ?? null
  } catch { return null }
})

export async function generateMetadata({ params, searchParams }: Props): Promise<Metadata> {
  const { slug: rawSlug } = await params
  const { customDomain } = await searchParams
  let slug = rawSlug
  if (customDomain) { const r = await resolveSlugFromCustomDomain(customDomain); if (r) slug = r }
  const data = await fetchStatusPage(slug)
  if (!data) return { title: "状态页未找到" }
  return {
    title: `${data.title} — 状态页`,
    description: `查看 ${data.title} 的实时服务可用性状态`,
    openGraph: { title: `${data.title} — 状态页`, description: `查看 ${data.title} 的实时服务可用性状态`, type: "website" },
  }
}

export default async function StatusPage({ params, searchParams }: Props) {
  const { slug: rawSlug } = await params
  const { customDomain } = await searchParams
  let slug = rawSlug
  if (customDomain) {
    const resolved = await resolveSlugFromCustomDomain(customDomain)
    if (resolved) slug = resolved
    else notFound()
  }
  const data = await fetchStatusPage(slug)
  if (!data) notFound()
  return <StatusClient data={data} />
}
