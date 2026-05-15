"use client"

import { useState } from "react"
import NodeSelector from "@/components/probe/NodeSelector"
import ProbeResults from "@/components/probe/ProbeResults"
import { probeSmtp } from "@/lib/api"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui"
import { Button } from "@/components/ui"
import { Input } from "@/components/ui"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui"

export default function SmtpProbeClient() {
  const [target, setTarget] = useState("")
  const [port, setPort] = useState("25")
  const [selectedNodes, setSelectedNodes] = useState<string[]>([])
  const [taskId, setTaskId] = useState<string | null>(null)
  const [submitError, setSubmitError] = useState("")
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!target.trim() || loading) return

    try {
      setSubmitError("")
      setTaskId(null)
      setLoading(true)
      const res = await probeSmtp({
        target: target.trim(),
        node_ids: selectedNodes.length > 0 ? selectedNodes : undefined,
        params: { port },
      })
      setTaskId(res.task_id)
    } catch (err) {
      setSubmitError(err instanceof Error ? err.message : "提交失败")
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">SMTP 邮件服务器检测</h1>
        <p className="text-muted-foreground mt-2">
          测试邮件服务器的 SMTP 连接性，验证 banner 和 EHLO 握手
        </p>
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        <div className="space-y-6">
          <Card>
            <CardHeader>
              <CardTitle>检测参数</CardTitle>
            </CardHeader>
            <CardContent>
              <form onSubmit={handleSubmit} className="space-y-4">
                <div className="space-y-2">
                  <label className="text-sm font-medium">邮件服务器地址</label>
                  <Input
                    value={target}
                    onChange={(e) => setTarget(e.target.value)}
                    placeholder="mail.example.com"
                    required
                  />
                </div>
                <div className="space-y-2">
                  <label className="text-sm font-medium">端口</label>
                  <Select value={port} onValueChange={setPort}>
                    <SelectTrigger>
                      <SelectValue placeholder="选择端口" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="25">25 (SMTP)</SelectItem>
                      <SelectItem value="465">465 (SMTPS)</SelectItem>
                      <SelectItem value="587">587 (Submission)</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <Button
                  type="submit"
                  disabled={!target.trim() || loading}
                  className="w-full"
                >
                  {loading ? "检测中..." : "开始检测"}
                </Button>
              </form>
            </CardContent>
          </Card>

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
          <p>• <strong>服务器地址</strong>：输入邮件服务器的域名或 IP（如 mail.example.com）</p>
          <p>• <strong>端口选择</strong>：25 为标准 SMTP，587 为邮件提交端口（推荐），465 为 SMTPS</p>
          <p>• <strong>检测内容</strong>：TCP 连接、220 banner 读取、EHLO 握手响应</p>
          <p>• <strong>节点选择</strong>：可选择特定节点，默认使用所有可用节点</p>
        </CardContent>
      </Card>
    </div>
  )
}
