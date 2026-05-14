import { Metadata } from "next"
import { NodesClient } from "./nodes-client"
import { MOCK_NODES } from "./mock-data"

export const metadata: Metadata = {
  title: "全球监控节点 - idcd",
  description: "idcd 全球分布的监控节点，覆盖中国大陆、香港、日本、新加坡、美国等地区",
}

export default function NodesPage() {
  return (
    <div className="min-h-screen bg-background">
      <div className="container mx-auto px-4 py-8">
        <div className="mb-8">
          <h1 className="text-3xl font-bold tracking-tight">全球监控节点</h1>
          <p className="mt-2 text-muted-foreground">
            idcd 在全球部署了多个监控节点，提供高质量的网络探测服务
          </p>
        </div>

        <NodesClient nodes={MOCK_NODES} />
      </div>
    </div>
  )
}
