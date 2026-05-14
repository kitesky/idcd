import type { Metadata } from "next"
import { NodesClient } from "./nodes-client"
import { MOCK_ADMIN_NODES } from "./mock-data"

export const metadata: Metadata = {
  title: "节点健康看板 — idcd Admin",
}

export default function NodesPage() {
  return (
    <div>
      <div className="mb-6">
        <h1 className="text-2xl font-bold tracking-tight">节点健康看板</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          全局节点在线状态、延迟和心跳监控（mock 数据）
        </p>
      </div>
      <NodesClient nodes={MOCK_ADMIN_NODES} />
    </div>
  )
}
