"use client"

import { useState } from "react"
import ProbeForm from "@/components/probe/ProbeForm"
import NodeSelector from "@/components/probe/NodeSelector"
import ProbeResults from "@/components/probe/ProbeResults"
import { probeTcp, type ProbeResult } from "@/lib/api"
import { Card, CardContent, CardHeader, CardTitle } from "@idcd/ui"

export default function TcpingProbeClient() {
  const [selectedNodes, setSelectedNodes] = useState<string[]>([])
  const [loading, setLoading] = useState(false)
  const [result, setResult] = useState<ProbeResult | null>(null)
  const [error, setError] = useState("")

  const handleSubmit = async (target: string, params: Record<string, any>) => {
    try {
      setLoading(true)
      setError("")
      const probeResult = await probeTcp({
        target,
        node_ids: selectedNodes.length > 0 ? selectedNodes : undefined,
        ...params
      })
      setResult(probeResult)
    } catch (err) {
      setError(err instanceof Error ? err.message : "拨测失败")
      setResult(null)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">TCP 端口连通性测试</h1>
        <p className="text-muted-foreground mt-2">
          从全球多个节点测试 TCP 端口的连通性和响应时间（TCPing）
        </p>
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        <div className="space-y-6">
          <ProbeForm type="tcp" onSubmit={handleSubmit} loading={loading} />
          <NodeSelector selectedNodes={selectedNodes} onNodesChange={setSelectedNodes} />
        </div>

        <div>
          <ProbeResults result={result} loading={loading} error={error} />
        </div>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>使用说明</CardTitle>
        </CardHeader>
        <CardContent className="text-sm text-muted-foreground space-y-2">
          <p>• <strong>目标地址</strong>：输入主机和端口（如 example.com:443 或 1.1.1.1:80）</p>
          <p>• <strong>常用端口</strong>：HTTP(80)、HTTPS(443)、SSH(22)、MySQL(3306)、Redis(6379)</p>
          <p>• <strong>测试内容</strong>：检测 TCP 三次握手是否成功，测量连接建立时间</p>
          <p>• <strong>节点选择</strong>：可选择特定节点，默认使用所有可用节点</p>
        </CardContent>
      </Card>
    </div>
  )
}
