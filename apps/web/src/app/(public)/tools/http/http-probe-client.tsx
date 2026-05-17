"use client"

import { useState } from "react"
import ProbeForm from "@/components/probe/ProbeForm"
import NodeSelector from "@/components/probe/NodeSelector"
import ProbeResults from "@/components/probe/ProbeResults"
import { probeHttp } from "@/lib/api"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui"

export default function HttpProbeClient() {
  const [selectedNodes, setSelectedNodes] = useState<string[]>([])
  const [taskId, setTaskId] = useState<string | null>(null)
  const [submitError, setSubmitError] = useState("")
  const [shareCtx, setShareCtx] = useState<{ target: string; params: Record<string, unknown> } | null>(null)

  const handleSubmit = async (target: string, params: Record<string, unknown>) => {
    try {
      setSubmitError("")
      setTaskId(null)
      setShareCtx(null)
      const res = await probeHttp({
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
        <h1 className="text-3xl font-bold">HTTP/HTTPS 拨测</h1>
        <p className="text-muted-foreground mt-2">
          从全球多个节点测试 HTTP/HTTPS 服务的可用性和响应时间
        </p>
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        <div className="space-y-6">
          <ProbeForm type="http" onSubmit={handleSubmit} loading={false} />
          <NodeSelector selectedNodes={selectedNodes} onNodesChange={setSelectedNodes} />
        </div>

        <div>
          <ProbeResults
            taskId={taskId}
            error={submitError}
            shareContext={shareCtx ? { tool: "http", ...shareCtx } : undefined}
          />
        </div>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>使用说明</CardTitle>
        </CardHeader>
        <CardContent className="text-sm text-muted-foreground space-y-2">
          <p>• <strong>目标地址</strong>：输入完整的 HTTP/HTTPS URL（如 https://example.com）</p>
          <p>• <strong>请求方法</strong>：选择 GET、POST 或 HEAD 方法</p>
          <p>• <strong>跟随重定向</strong>：是否自动跟随 3xx 重定向响应</p>
          <p>• <strong>节点选择</strong>：可选择特定节点，默认使用所有可用节点</p>
        </CardContent>
      </Card>
    </div>
  )
}
