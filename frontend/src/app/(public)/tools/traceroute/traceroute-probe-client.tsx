"use client"

import { useState } from "react"
import ProbeForm from "@/components/probe/ProbeForm"
import NodeSelector from "@/components/probe/NodeSelector"
import { TracerouteResultPanel } from "@/components/probe/TracerouteResultPanel"
import ShareResultButton from "@/components/probe/ShareResultButton"
import { useProbePolling } from "@/hooks/useProbePolling"
import { probeTraceroute } from "@/lib/api"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui"

export default function TracerouteProbeClient() {
  const [selectedNodes, setSelectedNodes] = useState<string[]>([])
  const [taskId, setTaskId] = useState<string | null>(null)
  const [submitError, setSubmitError] = useState("")
  const [shareCtx, setShareCtx] = useState<{ target: string; params: Record<string, unknown> } | null>(null)
  const polling = useProbePolling(taskId)

  const handleSubmit = async (target: string, params: Record<string, unknown>) => {
    try {
      setSubmitError("")
      setTaskId(null)
      setShareCtx(null)
      const res = await probeTraceroute({
        target,
        node_ids: selectedNodes.length > 0 ? selectedNodes : undefined,
        ...params,
      })
      setTaskId(res.task_id)
      setShareCtx({ target, params })
    } catch (err) {
      setSubmitError(err instanceof Error ? err.message : "提交失败")
    }
  }

  const shareSlot = shareCtx && polling.taskResult
    ? <ShareResultButton tool="traceroute" target={shareCtx.target} params={shareCtx.params} taskResult={polling.taskResult} />
    : null

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">路由追踪</h1>
        <p className="text-muted-foreground mt-2">
          从全球多个节点追踪到目标主机的网络路径，在地图上可视化每一跳的位置与延迟
        </p>
      </div>

      <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.5fr)]">
        <div className="space-y-6">
          <ProbeForm type="traceroute" onSubmit={handleSubmit} loading={polling.isPolling} />
          <NodeSelector selectedNodes={selectedNodes} onNodesChange={setSelectedNodes} />
        </div>

        <div>
          <TracerouteResultPanel
            polling={polling}
            sourceNodeId={selectedNodes[0] ?? undefined}
            variant="traceroute"
            error={submitError}
            headerSlot={shareSlot}
          />
        </div>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>使用说明</CardTitle>
        </CardHeader>
        <CardContent className="text-sm text-muted-foreground space-y-2">
          <p>• <strong>目标地址</strong>：输入域名或 IP 地址（如 example.com 或 1.1.1.1）</p>
          <p>• <strong>路径地图</strong>：基于每一跳的 IP 地理位置（GeoIP）连成路径，鼠标悬停查看详细信息</p>
          <p>• <strong>跳点列表</strong>：完整的 hop / IP / 主机名 / 位置 / 延迟表格</p>
          <p>• <strong>* 号或超时</strong>：路由器不响应 ICMP 或私网节点（如运营商网关），无法定位</p>
          <p>• <strong>节点选择</strong>：可选择起点节点，默认使用所有可用节点</p>
        </CardContent>
      </Card>
    </div>
  )
}
