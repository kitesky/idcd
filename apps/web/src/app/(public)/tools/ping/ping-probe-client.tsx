"use client"

import { useState } from "react"
import ProbeForm from "@/components/probe/ProbeForm"
import NodeSelector from "@/components/probe/NodeSelector"
import ProbeResults from "@/components/probe/ProbeResults"
import { probePing, type ProbeResult } from "@/lib/api"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui"

export default function PingProbeClient() {
  const [selectedNodes, setSelectedNodes] = useState<string[]>([])
  const [loading, setLoading] = useState(false)
  const [result, setResult] = useState<ProbeResult | null>(null)
  const [error, setError] = useState("")

  const handleSubmit = async (target: string, params: Record<string, any>) => {
    try {
      setLoading(true)
      setError("")
      const probeResult = await probePing({
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
        <h1 className="text-3xl font-bold">多地 Ping 测试</h1>
        <p className="text-muted-foreground mt-2">
          从全球多个节点 Ping 目标主机，检测网络延迟和连通性
        </p>
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        <div className="space-y-6">
          <ProbeForm type="ping" onSubmit={handleSubmit} loading={loading} />
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
          <p>• <strong>目标地址</strong>：输入域名或 IP 地址（如 example.com 或 1.1.1.1）</p>
          <p>• <strong>发送次数</strong>：选择发送 Ping 包的数量（4、10 或 20 次）</p>
          <p>• <strong>结果指标</strong>：显示平均延迟、最小/最大延迟和丢包率</p>
          <p>• <strong>节点选择</strong>：可选择特定节点，默认使用所有可用节点</p>
        </CardContent>
      </Card>
    </div>
  )
}
