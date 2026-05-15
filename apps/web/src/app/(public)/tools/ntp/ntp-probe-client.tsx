"use client"

import { useState } from "react"
import NodeSelector from "@/components/probe/NodeSelector"
import ProbeResults from "@/components/probe/ProbeResults"
import { probeNtp } from "@/lib/api"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui"
import { Button } from "@/components/ui"
import { Input } from "@/components/ui"

export default function NtpProbeClient() {
  const [target, setTarget] = useState("")
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
      const res = await probeNtp({
        target: target.trim(),
        node_ids: selectedNodes.length > 0 ? selectedNodes : undefined,
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
        <h1 className="text-3xl font-bold">NTP 服务器检测</h1>
        <p className="text-muted-foreground mt-2">
          查询 NTP 时间服务器，返回服务器时间与本地时钟的偏移量
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
                  <label className="text-sm font-medium">NTP 服务器地址</label>
                  <Input
                    value={target}
                    onChange={(e) => setTarget(e.target.value)}
                    placeholder="pool.ntp.org 或 time.cloudflare.com"
                    required
                  />
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
          <p>• <strong>服务器地址</strong>：输入 NTP 服务器的域名或 IP（如 pool.ntp.org、time.apple.com）</p>
          <p>• <strong>检测内容</strong>：UDP port 123 连接，读取服务器时间戳，计算时钟偏移量</p>
          <p>• <strong>偏移量说明</strong>：正值表示服务器时间超前本地，负值表示落后</p>
          <p>• <strong>节点选择</strong>：可选择特定节点，默认使用所有可用节点</p>
        </CardContent>
      </Card>
    </div>
  )
}
