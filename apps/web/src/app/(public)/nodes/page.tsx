import { Metadata } from "next"
import { NodesClient } from "./nodes-client"
import type { NodeEntry } from "@/lib/nodes-utils"

export const metadata: Metadata = {
  title: "全球监控节点 - idcd",
  description: "idcd 全球分布的监控节点，覆盖中国大陆、香港、日本、新加坡、美国等地区",
}

interface ApiNode {
  id: string
  name: string
  tier?: string
  country_code?: string
  region?: string
  city?: string
  isp?: string
  ip?: string
  status: "active" | "inactive" | "degraded"
  uptime_percent?: number
  latency_ms?: number
  last_seen_at?: string
}

function mapApiNode(n: ApiNode): NodeEntry {
  return {
    id: n.id,
    asn: "",
    carrier: n.isp ?? "",
    region: n.city || n.region || "",
    exitIp: n.ip ?? "",
    status: n.status === "active" ? "online" : n.status === "degraded" ? "degraded" : "offline",
    country: n.country_code ?? "",
  }
}

async function fetchNodes(): Promise<{ nodes: NodeEntry[]; error?: string }> {
  try {
    const res = await fetch(
      `${process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080"}/v1/nodes`,
      { next: { revalidate: 60 } }
    )
    if (!res.ok) {
      return { nodes: [], error: `获取节点数据失败（HTTP ${res.status}）` }
    }
    const json = await res.json()
    const apiNodes: ApiNode[] = json?.data?.nodes ?? []
    return { nodes: apiNodes.map(mapApiNode) }
  } catch {
    return { nodes: [], error: "无法连接到后端服务，请稍后重试" }
  }
}

export default async function NodesPage() {
  const { nodes, error } = await fetchNodes()

  return (
    <div className="min-h-screen bg-background">
      <div className="container mx-auto px-4 py-8">
        <div className="mb-8">
          <h1 className="text-3xl font-bold tracking-tight">全球监控节点</h1>
          <p className="mt-2 text-muted-foreground">
            idcd 在全球部署了多个监控节点，提供高质量的网络探测服务
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
