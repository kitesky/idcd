import type { Metadata } from "next"
import { NodesClient } from "./nodes-client"
import type { AdminNode } from "./nodes-client"

export const metadata: Metadata = { title: "节点健康看板 — idcd Admin" }

async function fetchNodes(): Promise<AdminNode[]> {
  const base = process.env.INTERNAL_API_URL ?? "http://localhost:8080"
  const token = process.env.ADMIN_TOKEN ?? ""
  try {
    const res = await fetch(`${base}/internal/admin/nodes`, {
      headers: { "X-Admin-Token": token },
      cache: "no-store",
    })
    if (!res.ok) return []
    return res.json()
  } catch {
    return []
  }
}

export default async function NodesPage() {
  const nodes = await fetchNodes()

  return (
    <div>
      <div className="mb-6">
        <h1 className="text-2xl font-bold tracking-tight">节点健康看板</h1>
        <p className="mt-1 text-sm text-muted-foreground">全局节点在线状态、延迟和心跳监控</p>
      </div>
      <NodesClient nodes={nodes} />
    </div>
  )
}
