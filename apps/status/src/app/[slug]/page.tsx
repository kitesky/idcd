import type { Metadata } from "next"
import { notFound } from "next/navigation"
import { MOCK_STATUS_PAGES } from "./mock-data"
import { StatusClient } from "./status-client"

// Revalidate every 60 seconds so the status page reflects near-real-time data
// without blocking every request. Adjust downward once real data is wired in.
export const revalidate = 60

interface Props {
  params: Promise<{ slug: string }>
  searchParams: Promise<{ customDomain?: string }>
}

export async function generateStaticParams() {
  return Object.keys(MOCK_STATUS_PAGES).map((slug) => ({ slug }))
}

/**
 * Resolve the status page slug from a custom domain by calling the internal
 * API endpoint. Returns the slug on success, or null when not found / not
 * yet verified.
 */
async function resolveSlugFromCustomDomain(
  customDomain: string,
): Promise<string | null> {
  const apiBase =
    process.env.INTERNAL_API_URL ?? "http://localhost:8080"
  try {
    const res = await fetch(
      `${apiBase}/internal/status-pages/by-domain?domain=${encodeURIComponent(customDomain)}`,
      { cache: "no-store" },
    )
    if (!res.ok) return null
    const json = (await res.json()) as { data?: { slug?: string } }
    return json.data?.slug ?? null
  } catch {
    return null
  }
}

export async function generateMetadata({ params, searchParams }: Props): Promise<Metadata> {
  const { slug: rawSlug } = await params
  const { customDomain } = await searchParams

  let slug = rawSlug
  if (customDomain) {
    const resolved = await resolveSlugFromCustomDomain(customDomain)
    if (resolved) slug = resolved
  }

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

export default async function StatusPage({ params, searchParams }: Props) {
  const { slug: rawSlug } = await params
  const { customDomain } = await searchParams

  let slug = rawSlug

  // When the request comes from a custom domain, resolve the actual slug.
  if (customDomain) {
    const resolved = await resolveSlugFromCustomDomain(customDomain)
    if (resolved) {
      slug = resolved
    } else {
      // Custom domain exists in URL params but couldn't be resolved → 404.
      notFound()
    }
  }

  const data = MOCK_STATUS_PAGES[slug]
  if (!data) {
    notFound()
  }

  return <StatusClient data={data} />
}
