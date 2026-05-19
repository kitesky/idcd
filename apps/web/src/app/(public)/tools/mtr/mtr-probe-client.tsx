"use client"

import { useState } from "react"
import ProbeForm from "@/components/probe/ProbeForm"
import NodeSelector from "@/components/probe/NodeSelector"
import { TracerouteResultPanel } from "@/components/probe/TracerouteResultPanel"
import { useProbePolling } from "@/hooks/useProbePolling"
import { probeMtr } from "@/lib/api"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui"

export default function MTRProbeClient() {
  const [selectedNodes, setSelectedNodes] = useState<string[]>([])
  const [taskId, setTaskId] = useState<string | null>(null)
  const [submitError, setSubmitError] = useState("")
  const polling = useProbePolling(taskId)

  const handleSubmit = async (target: string, params: Record<string, unknown>) => {
    try {
      setSubmitError("")
      setTaskId(null)
      const res = await probeMtr({
        target,
        node_ids: selectedNodes.length > 0 ? selectedNodes : undefined,
        ...params,
      })
      setTaskId(res.task_id)
    } catch (err) {
      setSubmitError(err instanceof Error ? err.message : "提交失败")
    }
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">MTR 路由测试</h1>
        <p className="text-muted-foreground mt-2">
          结合 Ping 和 Traceroute 功能，对路径上每一跳进行多次测量并在地图上展示路径与丢包
        </p>
      </div>

      <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.5fr)]">
        <div className="space-y-6">
          <ProbeForm type="mtr" onSubmit={handleSubmit} loading={polling.isPolling} />
          <NodeSelector selectedNodes={selectedNodes} onNodesChange={setSelectedNodes} />
        </div>

        <div>
          <TracerouteResultPanel
            polling={polling}
            sourceNodeId={selectedNodes[0]}
            variant="mtr"
            error={submitError}
          />
        </div>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>使用说明</CardTitle>
        </CardHeader>
        <CardContent className="text-sm text-muted-foreground space-y-2">
          <p>• <strong>目标地址</strong>：输入域名或 IP 地址（如 example.com 或 1.1.1.1）</p>
          <p>• <strong>工作原理</strong>：先通过 Traceroute 探测路径，再对每个跳点发送 3 次 Ping，统计延迟和丢包</p>
          <p>• <strong>路径地图</strong>：基于 GeoIP 在世界地图上连出每一跳，悬停查看 IP / 位置 / RTT</p>
          <p>• <strong>跳点列表</strong>：完整 hop / IP / 主机名 / 丢包率 / 平均延迟</p>
          <p>• <strong>* 号跳点</strong>：该路由器不响应 ICMP，属正常现象</p>
          <p>• <strong>应用场景</strong>：精确定位网络瓶颈、判断是哪一跳导致丢包或高延迟</p>
        </CardContent>
      </Card>
    </div>
  )
}
