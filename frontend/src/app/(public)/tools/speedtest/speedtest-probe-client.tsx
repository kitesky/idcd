"use client"

import { useState } from "react"
import { Loader2Icon, GaugeIcon, ZapIcon } from "lucide-react"
import NodeSelector from "@/components/probe/NodeSelector"
import ProbeResults from "@/components/probe/ProbeResults"
import { probeSpeedtest } from "@/lib/api"
import { Card, CardContent, CardHeader, CardTitle, Button, Input, Label, Badge } from "@/components/ui"
import { useProbePolling } from "@/hooks/useProbePolling"

export default function SpeedtestProbeClient() {
  const [target, setTarget] = useState("")
  const [selectedNodes, setSelectedNodes] = useState<string[]>([])
  const [taskId, setTaskId] = useState<string | null>(null)
  const [submitError, setSubmitError] = useState("")
  const [loading, setLoading] = useState(false)

  const polling = useProbePolling(taskId)
  const { taskResult } = polling

  const downloadMbps = taskResult?.result
    ? (taskResult.result as Record<string, unknown>)["download_mbps"] as number | undefined
    : undefined
  const uploadMbps = taskResult?.result
    ? (taskResult.result as Record<string, unknown>)["upload_mbps"] as number | undefined
    : undefined

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!target.trim() || loading) return

    try {
      setSubmitError("")
      setTaskId(null)
      setLoading(true)
      const res = await probeSpeedtest({
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
        <h1 className="text-3xl font-bold">网速测试</h1>
        <p className="text-muted-foreground mt-2">
          通过 HTTP 大包下载/上传测量目标服务器带宽，返回 download_mbps / upload_mbps
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
                  <Label htmlFor="speedtest-target">目标 URL</Label>
                  <Input
                    id="speedtest-target"
                    value={target}
                    onChange={(e) => setTarget(e.target.value)}
                    placeholder="https://speed.cloudflare.com/__down?bytes=10000000"
                    required
                  />
                  <p className="text-xs text-muted-foreground">
                    支持 http(s):// URL 或裸域名（自动补 https://）
                  </p>
                </div>
                <Button
                  type="submit"
                  disabled={!target.trim() || loading || polling.isPolling}
                  className="w-full"
                >
                  {loading ? (
                    <>
                      <Loader2Icon className="size-4 animate-spin" aria-hidden="true" />
                      <span>正在提交</span>
                    </>
                  ) : polling.isPolling ? (
                    <>
                      <GaugeIcon className="size-4 animate-pulse" aria-hidden="true" />
                      <span>测速进行中</span>
                    </>
                  ) : (
                    <>
                      <ZapIcon className="size-4" aria-hidden="true" />
                      <span>开始测速</span>
                    </>
                  )}
                </Button>
              </form>
            </CardContent>
          </Card>

          <NodeSelector selectedNodes={selectedNodes} onNodesChange={setSelectedNodes} />

          {/* Speed gauge cards — shown once we have results */}
          {(downloadMbps !== undefined || uploadMbps !== undefined) && (
            <div className="grid grid-cols-2 gap-4">
              <Card>
                <CardHeader className="pb-2">
                  <CardTitle className="text-sm font-medium text-muted-foreground">下载速度</CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="text-2xl font-bold">
                    {downloadMbps !== undefined ? downloadMbps.toFixed(1) : "—"}
                  </div>
                  <Badge variant="secondary" className="mt-1">Mbps</Badge>
                </CardContent>
              </Card>
              <Card>
                <CardHeader className="pb-2">
                  <CardTitle className="text-sm font-medium text-muted-foreground">上传速度</CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="text-2xl font-bold">
                    {uploadMbps !== undefined ? uploadMbps.toFixed(1) : "—"}
                  </div>
                  <Badge variant="secondary" className="mt-1">Mbps</Badge>
                </CardContent>
              </Card>
            </div>
          )}
        </div>

        <div>
          <ProbeResults taskId={taskId} polling={polling} error={submitError} />
        </div>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>使用说明</CardTitle>
        </CardHeader>
        <CardContent className="text-sm text-muted-foreground space-y-2">
          <p>• <strong>目标 URL</strong>：填入支持 Range 请求的下载端点，如 Cloudflare Speed Test</p>
          <p>• <strong>下载测速</strong>：发送 GET + Range 请求，读取并丢弃数据，计算实际带宽</p>
          <p>• <strong>上传测速</strong>：发送 1 MiB 随机数据 POST，计算上传带宽</p>
          <p>• <strong>节点选择</strong>：可选择特定节点，默认使用所有可用节点</p>
        </CardContent>
      </Card>
    </div>
  )
}
