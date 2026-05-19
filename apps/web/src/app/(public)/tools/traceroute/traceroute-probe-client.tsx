"use client"

import { useState } from "react"
import ProbeForm from "@/components/probe/ProbeForm"
import NodeSelector from "@/components/probe/NodeSelector"
import ProbeResults from "@/components/probe/ProbeResults"
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

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">路由追踪</h1>
        <p className="text-muted-foreground mt-2">
          从全球多个节点追踪到目标主机的网络路径，显示每一跳的延迟
        </p>
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        <div className="space-y-6">
          <ProbeForm type="traceroute" onSubmit={handleSubmit} loading={polling.isPolling} />
          <NodeSelector selectedNodes={selectedNodes} onNodesChange={setSelectedNodes} />
        </div>

        <div>
          <ProbeResults
            taskId={taskId}
            polling={polling}
            error={submitError}
            shareContext={shareCtx ? { tool: "traceroute", ...shareCtx } : undefined}
          />
        </div>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>使用说明</CardTitle>
        </CardHeader>
        <CardContent className="text-sm text-muted-foreground space-y-2">
          <p>• <strong>目标地址</strong>：输入域名或 IP 地址（如 example.com 或 1.1.1.1）</p>
          <p>• <strong>路由信息</strong>：显示数据包经过的每一跳路由器的 IP 和延迟</p>
          <p>• <strong>应用场景</strong>：诊断网络路径、定位网络瓶颈、分析跨国线路</p>
          <p>• <strong>节点选择</strong>：可选择特定节点，默认使用所有可用节点</p>
        </CardContent>
      </Card>
    </div>
  )
}
