import { Metadata } from "next"
import { getT } from "@/i18n/getT"
import { NodesClient } from "./nodes-client"
import type { NodeEntry } from "@/lib/nodes-utils"
import type { Node } from "@/lib/api"

export async function generateMetadata(): Promise<Metadata> {
  const t = await getT("nodes")
  return {
    title: `${t("publicTitle")} - idcd`,
    description: t("publicSubtitle"),
  }
}

function mapApiNode(n: Node): NodeEntry {
  return {
    id: n.id,
    name: n.name || n.city || n.region || n.country_code || "",
    asn: n.asn ?? "",
    carrier: n.isp ?? "",
    region: n.city || n.region || "",
    exitIp: "",
    status: n.status === "active" ? "online" : n.status === "degraded" ? "degraded" : "offline",
    country: n.country_code ?? "",
  }
}

async function fetchNodes(t: (key: string, params?: Record<string, string | number>) => string): Promise<{ nodes: NodeEntry[]; error?: string }> {
  try {
    const res = await fetch(
      `${process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080"}/v1/nodes`,
      { next: { revalidate: 60 } }
    )
    if (!res.ok) {
      return { nodes: [], error: t("fetchError", { status: String(res.status) }) }
    }
    const json = await res.json()
    const apiNodes: Node[] = json?.data?.nodes ?? []
    return { nodes: apiNodes.map(mapApiNode) }
  } catch {
    return { nodes: [], error: t("fetchNetworkError") }
  }
}

export default async function NodesPage() {
  const t = await getT("nodes")
  const { nodes, error } = await fetchNodes(t)

  return (
    <div className="min-h-screen bg-background">
      <div className="mx-auto max-w-screen-xl px-4 sm:px-6 lg:px-8 py-12 md:py-16">
        <div className="mb-8">
          <h1 className="text-2xl font-bold tracking-tight">{t("publicTitle")}</h1>
          <p className="mt-1.5 text-sm text-muted-foreground">
            {t("publicSubtitle")}
          </p>
        </div>

        {error && (
          <div
            data-testid="fetch-error"
            className="mb-6 rounded-md border border-destructive/50 bg-destructive/10 px-4 py-3 text-sm text-destructive"
          >
            {error}
          </div>
        )}

        <NodesClient nodes={nodes} />
      </div>
    </div>
  )
}
