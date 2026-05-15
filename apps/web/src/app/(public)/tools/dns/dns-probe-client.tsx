"use client"

import { useState } from "react"
import ProbeForm from "@/components/probe/ProbeForm"
import NodeSelector from "@/components/probe/NodeSelector"
import ProbeResults from "@/components/probe/ProbeResults"
import { probeDns } from "@/lib/api"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui"

export default function DnsProbeClient() {
  const [selectedNodes, setSelectedNodes] = useState<string[]>([])
  const [taskId, setTaskId] = useState<string | null>(null)
  const [submitError, setSubmitError] = useState("")

  const handleSubmit = async (target: string, params: Record<string, unknown>) => {
    try {
      setSubmitError("")
      setTaskId(null)
      const res = await probeDns({
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
        <h1 className="text-3xl font-bold">DNS 解析查询</h1>
        <p className="text-muted-foreground mt-2">
          从全球多个节点查询 DNS 记录，检测域名解析结果和响应时间
        </p>
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        <div className="space-y-6">
          <ProbeForm type="dns" onSubmit={handleSubmit} loading={false} />
          <NodeSelector selectedNodes={selectedNodes} onNodesChange={setSelectedNodes} />
        </div>

        <div>
          <ProbeResults taskId={taskId} error={submitError} />
        </div>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>使用说明</CardTitle>
        </CardHeader>
        <CardContent className="text-sm text-muted-foreground space-y-2">
          <p>• <strong>目标域名</strong>：输入要查询的域名（如 example.com）</p>
          <p>• <strong>记录类型</strong>：A(IPv4)、AAAA(IPv6)、MX(邮件)、TXT(文本)、CNAME(别名)、NS(域名服务器)</p>
          <p>• <strong>查询内容</strong>：显示 DNS 解析结果、响应时间和权威服务器</p>
          <p>• <strong>节点选择</strong>：可选择特定节点，默认使用所有可用节点</p>
        </CardContent>
      </Card>
    </div>
  )
}
