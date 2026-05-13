import { Metadata } from "next"
import { NodesClient } from "./nodes-client"
import { getNodes } from "@/lib/api"

export const metadata: Metadata = {
  title: "全球监控节点 - idcd",
  description: "idcd 全球分布的监控节点，覆盖中国大陆、香港、日本、新加坡、美国等地区",
}

export default async function NodesPage() {
  let nodes: any[] = []
  let error: string | null = null

  try {
    const result = await getNodes()
    nodes = result.data || []
  } catch (e) {
    error = e instanceof Error ? e.message : "加载节点数据失败"
  }

  return (
    <div className="min-h-screen bg-background">
      <div className="container mx-auto px-4 py-8">
        {/* 页面标题 */}
        <div className="mb-8">
          <h1 className="text-4xl font-bold mb-2">全球监控节点</h1>
          <p className="text-muted-foreground">
            idcd 在全球部署了多个监控节点，提供高质量的网络探测服务
          </p>
        </div>

        {error ? (
          <div className="rounded-lg border border-destructive bg-destructive/10 p-4">
            <p className="text-destructive">{error}</p>
          </div>
        ) : (
          <NodesClient initialNodes={nodes} />
        )}
      </div>
    </div>
  )
}
