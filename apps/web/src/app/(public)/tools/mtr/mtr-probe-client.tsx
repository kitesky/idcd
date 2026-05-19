"use client"

import { useState } from "react"
import ProbeForm from "@/components/probe/ProbeForm"
import NodeSelector from "@/components/probe/NodeSelector"
import ProbeResults from "@/components/probe/ProbeResults"
import { probeMtr, type ProbeTaskResult } from "@/lib/api"
import { useProbePolling } from "@/hooks/useProbePolling"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui"
import { Badge } from "@/components/ui/badge"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"

interface MTRHop {
  hop: number
  ip: string
  hostname?: string
  sent_pkts: number
  recv_pkts: number
  loss_pct: number
  avg_rtt_ms: number
  min_rtt_ms: number
  max_rtt_ms: number
  timeout?: boolean
}

interface MTRData {
  hops?: MTRHop[]
  total_hops?: number
  target_reached?: boolean
}

function LossBadge({ loss }: { loss: number }) {
  if (loss === 0) {
    return <Badge variant="outline" className="text-green-600 border-green-500">0%</Badge>
  }
  if (loss < 10) {
    return <Badge variant="outline" className="text-yellow-600 border-yellow-500">{loss.toFixed(1)}%</Badge>
  }
  if (loss < 50) {
    return <Badge variant="outline" className="text-orange-600 border-orange-500">{loss.toFixed(1)}%</Badge>
  }
  return <Badge variant="destructive">{loss.toFixed(1)}%</Badge>
}

function MTRHopTable({ hops }: { hops: MTRHop[] }) {
  if (!hops || hops.length === 0) return null

  return (
    <div className="overflow-x-auto rounded-md border">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className="w-12 text-center">跳</TableHead>
            <TableHead>IP 地址</TableHead>
            <TableHead>主机名</TableHead>
            <TableHead className="text-right">丢包率</TableHead>
            <TableHead className="text-right">发送</TableHead>
            <TableHead className="text-right">接收</TableHead>
            <TableHead className="text-right">最小延迟</TableHead>
            <TableHead className="text-right">平均延迟</TableHead>
            <TableHead className="text-right">最大延迟</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {hops.map((hop) => (
            <TableRow key={hop.hop}>
              <TableCell className="text-center font-mono text-sm">{hop.hop}</TableCell>
              <TableCell className="font-mono text-sm">
                {hop.timeout || !hop.ip ? (
                  <span className="text-muted-foreground">*</span>
                ) : (
                  hop.ip
                )}
              </TableCell>
              <TableCell className="text-sm text-muted-foreground max-w-[200px] truncate">
                {hop.hostname ?? "—"}
              </TableCell>
              <TableCell className="text-right">
                {hop.timeout ? (
                  <Badge variant="destructive">100%</Badge>
                ) : (
                  <LossBadge loss={hop.loss_pct} />
                )}
              </TableCell>
              <TableCell className="text-right font-mono text-sm">{hop.sent_pkts}</TableCell>
              <TableCell className="text-right font-mono text-sm">{hop.recv_pkts}</TableCell>
              <TableCell className="text-right font-mono text-sm">
                {hop.timeout ? "—" : `${hop.min_rtt_ms.toFixed(1)} ms`}
              </TableCell>
              <TableCell className="text-right font-mono text-sm">
                {hop.timeout ? "—" : `${hop.avg_rtt_ms.toFixed(1)} ms`}
              </TableCell>
              <TableCell className="text-right font-mono text-sm">
                {hop.timeout ? "—" : `${hop.max_rtt_ms.toFixed(1)} ms`}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}

function MTRHopSection({ taskResult }: { taskResult: ProbeTaskResult | null }) {
  if (!taskResult?.result) return null

  // The probe result data is nested under result.data for MTR.
  const raw = taskResult.result as Record<string, unknown>
  const data = (raw["data"] ?? raw) as MTRData

  if (!data?.hops || data.hops.length === 0) return null

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          逐跳统计
          {data.target_reached !== undefined && (
            <Badge variant={data.target_reached ? "default" : "destructive"}>
              {data.target_reached ? "已到达目标" : "未到达目标"}
            </Badge>
          )}
          {data.total_hops !== undefined && (
            <span className="text-sm font-normal text-muted-foreground">
              共 {data.total_hops} 跳
            </span>
          )}
        </CardTitle>
      </CardHeader>
      <CardContent>
        <MTRHopTable hops={data.hops} />
      </CardContent>
    </Card>
  )
}

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
          结合 Ping 和 Traceroute 功能，对路径上每一跳进行多次测量，统计延迟和丢包率
        </p>
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        <div className="space-y-6">
          <ProbeForm type="mtr" onSubmit={handleSubmit} loading={polling.isPolling} />
          <NodeSelector selectedNodes={selectedNodes} onNodesChange={setSelectedNodes} />
        </div>

        <div>
          <ProbeResults taskId={taskId} polling={polling} error={submitError} />
        </div>
      </div>

      <MTRHopSection taskResult={polling.taskResult} />

      <Card>
        <CardHeader>
          <CardTitle>使用说明</CardTitle>
        </CardHeader>
        <CardContent className="text-sm text-muted-foreground space-y-2">
          <p>• <strong>目标地址</strong>：输入域名或 IP 地址（如 example.com 或 1.1.1.1）</p>
          <p>• <strong>工作原理</strong>：先通过 Traceroute 探测路径，再对每个跳点发送 3 次 Ping，统计延迟和丢包</p>
          <p>• <strong>丢包率颜色</strong>：绿色 0%、黄色 &lt;10%、橙色 &lt;50%、红色 ≥50%</p>
          <p>• <strong>* 号跳点</strong>：该路由器不响应 ICMP，属正常现象</p>
          <p>• <strong>应用场景</strong>：精确定位网络瓶颈、判断是哪一跳导致丢包或高延迟</p>
        </CardContent>
      </Card>
    </div>
  )
}
