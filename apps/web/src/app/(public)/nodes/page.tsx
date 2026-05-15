import { Metadata } from "next"
import { NodesClient } from "./nodes-client"
import type { NodeEntry } from "@/lib/nodes-utils"
import type { Node } from "@/lib/api"

export const metadata: Metadata = {
  title: "全球监控节点 - idcd",
  description: "idcd 全球分布的监控节点，覆盖中国大陆、香港、日本、新加坡、美国等地区",
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
    const apiNodes: Node[] = json?.data?.nodes ?? []
    return { nodes: apiNodes.map(mapApiNode) }
  } catch {
    return { nodes: [], error: "无法连接到后端服务，请稍后重试" }
  }
}

export default async function NodesPage() {
  const { nodes, error } = await fetchNodes()

  return (
    <div className="min-h-screen bg-background">
      <div className="mx-auto max-w-screen-xl px-4 sm:px-6 lg:px-8 py-12 md:py-16">
        <div className="mb-8">
          <h1 className="text-2xl font-bold tracking-tight">全球监控节点</h1>
          <p className="mt-1.5 text-sm text-muted-foreground">
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
