"use client"

import { useEffect, useState } from "react"
import { Loader2Icon, RadioTowerIcon } from "lucide-react"
import { Card, CardContent, CardHeader, CardTitle, Badge } from "@/components/ui"
import { useProbePolling } from "@/hooks/useProbePolling"
import { getNodes, type Node, type ProbeResult, type ProbeTaskResult } from "@/lib/api"
import type { SingleProbeReport } from "@/lib/diagnose-store"
import ShareResultButton from "./ShareResultButton"

// Module-level cache so multiple ProbeResults instances (one per tool page,
// plus the diagnose flow's parallel calls) share a single /v1/nodes fetch
// instead of stampeding it. Stale data is acceptable here — node names are
// effectively static and the worst case is showing "node-1" instead of the
// renamed hostname for a few minutes.
let nodesCache: Node[] | null = null
let nodesPromise: Promise<Node[]> | null = null
function fetchNodesOnce(): Promise<Node[]> {
  if (nodesCache) return Promise.resolve(nodesCache)
  if (!nodesPromise) {
    nodesPromise = getNodes()
      .then((n) => { nodesCache = n; return n })
      .catch((err) => { nodesPromise = null; throw err })
  }
  return nodesPromise
}

// PollingState mirrors the shape returned by useProbePolling so callers can
// either let ProbeResults drive its own polling (legacy single-tool pages and
// tests) or lift the hook to the page so the submit button can react to
// isPolling — see *-probe-client.tsx in (public)/tools/*.
type PollingState = {
  taskResult: ProbeTaskResult | null
  isPolling: boolean
  error: string
}

interface ProbeResultsProps {
  result?: ProbeResult | null  // legacy prop mode
  taskId?: string | null       // new polling mode (managed internally)
  // When provided, ProbeResults consumes this polling state instead of
  // calling useProbePolling itself — required to share polling state with
  // the ProbeForm submit button.
  polling?: PollingState
  loading?: boolean
  error?: string
  // When set, render a "Share Result" button once the task reaches a terminal state.
  shareContext?: {
    tool: SingleProbeReport["tool"]
    target: string
    params?: Record<string, unknown>
  }
}

interface DisplayItem {
  node_id: string
  node_name?: string
  success: boolean
  latency_ms?: number
  error?: string
  details?: Record<string, unknown>
}

// Human-readable labels for the most common probe-payload fields. Anything
// not in the table falls back to the raw key, so adding a new probe type
// doesn't break this — it just shows a slightly less polished label.
const DETAIL_LABELS: Record<string, string> = {
  packets_sent: "发送",
  packets_received: "接收",
  packet_loss: "丢包",
  min_ms: "最小",
  avg_ms: "平均",
  max_ms: "最大",
  stddev_ms: "抖动",
  status_code: "状态码",
  dns_lookup_ms: "DNS",
  tcp_connect_ms: "TCP",
  tls_handshake_ms: "TLS",
  ttfb_ms: "TTFB",
  download_mbps: "下载",
  upload_mbps: "上传",
  records: "记录",
  total_hops: "跳数",
  target_reached: "到达",
}

function formatDetailValue(key: string, value: unknown): string {
  if (value === null || value === undefined) return "-"
  if (typeof value === "number") {
    if (key === "packet_loss") return `${value}%`
    if (key.endsWith("_ms")) return `${value} ms`
    if (key.endsWith("_mbps")) return `${value.toFixed(1)} Mbps`
    return String(value)
  }
  if (typeof value === "boolean") return value ? "是" : "否"
  if (Array.isArray(value)) return `${value.length} 条`
  if (typeof value === "object") return JSON.stringify(value)
  return String(value)
}

function buildDisplayItems(
  result: ProbeResult | null | undefined,
  taskResult: ProbeTaskResult | null | undefined
): DisplayItem[] {
  // Legacy result with results array
  if (result?.results && result.results.length > 0) return result.results

  // New polling result
  if (taskResult?.result) {
    const r = taskResult.result
    const knownKeys = new Set(["node_id", "success", "duration_ms", "error"])
    const details: Record<string, unknown> = Object.fromEntries(
      Object.entries(r).filter(([k]) => !knownKeys.has(k))
    )
    return [{
      node_id: r.node_id ?? "unknown",
      success: r.success ?? false,
      latency_ms: r.duration_ms,
      error: r.error,
      details: Object.keys(details).length > 0 ? details : undefined,
    }]
  }
  return []
}

export default function ProbeResults({
  result,
  taskId,
  polling,
  loading = false,
  error: externalError,
  shareContext,
}: ProbeResultsProps) {
  // Always call the hook (rules of hooks); pass null when the parent already
  // owns the polling state via `polling` so we don't double-poll the API.
  const internal = useProbePolling(polling ? null : taskId ?? null)
  const { taskResult, isPolling, error: pollError } = polling ?? internal

  const [nodeNameById, setNodeNameById] = useState<Record<string, string>>({})
  useEffect(() => {
    let cancelled = false
    fetchNodesOnce()
      .then((nodes) => {
        if (cancelled) return
        const map: Record<string, string> = {}
        for (const n of nodes) map[n.id] = n.name || n.id
        setNodeNameById(map)
      })
      .catch(() => { /* fall back to raw node_id */ })
    return () => { cancelled = true }
  }, [])

  const isLoading = loading || isPolling
  const error = externalError || pollError

  const rawItems = buildDisplayItems(result, taskResult)
  const displayItems = rawItems.map((it) => ({
    ...it,
    node_name: it.node_name ?? nodeNameById[it.node_id],
  }))
  const successCount = displayItems.filter(r => r.success).length
  const totalCount = displayItems.length

  if (isLoading && totalCount === 0) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>拨测结果</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-3">
            {[1, 2, 3].map((i) => (
              <div key={i} className="h-16 bg-muted/50 animate-pulse rounded-md" />
            ))}
          </div>
          {isPolling && (
            <div className="mt-4 flex items-center gap-2 text-xs text-muted-foreground">
              <Loader2Icon className="size-3.5 animate-spin" aria-hidden="true" />
              <span>等待节点返回结果...</span>
            </div>
          )}
        </CardContent>
      </Card>
    )
  }

  if (error) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>拨测结果</CardTitle>
        </CardHeader>
        <CardContent>
          <Badge variant="destructive">错误：{error}</Badge>
        </CardContent>
      </Card>
    )
  }

  if (!result && !taskResult) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>拨测结果</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-muted-foreground text-sm">
            提交拨测后，结果将显示在此处
          </p>
        </CardContent>
      </Card>
    )
  }

  const shownTaskId = result?.task_id ?? taskResult?.task_id

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle>拨测结果</CardTitle>
          <div className="flex items-center gap-3">
            {totalCount > 0 && (
              <Badge variant={successCount === totalCount ? "default" : "secondary"}>
                成功 {successCount} / {totalCount}
              </Badge>
            )}
            {shareContext && taskResult && (
              <ShareResultButton
                tool={shareContext.tool}
                target={shareContext.target}
                params={shareContext.params}
                taskResult={taskResult}
              />
            )}
          </div>
        </div>
      </CardHeader>
      <CardContent>
        {isPolling && (
          <div className="mb-4">
            <Badge className="gap-1.5">
              <RadioTowerIcon className="size-3 animate-pulse" aria-hidden="true" />
              正在获取结果
            </Badge>
          </div>
        )}

        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b">
                <th className="text-left py-2 px-3">节点</th>
                <th className="text-left py-2 px-3">状态</th>
                <th className="text-left py-2 px-3">耗时</th>
                <th className="text-left py-2 px-3">详情</th>
              </tr>
            </thead>
            <tbody>
              {displayItems.length === 0 ? (
                <tr>
                  <td colSpan={4} className="text-center py-6 text-muted-foreground">
                    暂无结果
                  </td>
                </tr>
              ) : (
                displayItems.map((item, index) => (
                  <tr key={index} className="border-b">
                    <td className="py-3 px-3">
                      <div className="font-medium">{item.node_name ?? item.node_id}</div>
                    </td>
                    <td className="py-3 px-3">
                      <Badge variant={item.success ? "default" : "destructive"}>
                        {item.success ? "成功" : "失败"}
                      </Badge>
                    </td>
                    <td className="py-3 px-3">
                      {item.latency_ms !== undefined ? `${item.latency_ms} ms` : "-"}
                    </td>
                    <td className="py-3 px-3 text-muted-foreground">
                      {item.error && (
                        <div className="mb-1 text-destructive break-all">{item.error}</div>
                      )}
                      {item.details && Object.keys(item.details).length > 0 ? (
                        <div className="flex flex-wrap gap-x-3 gap-y-1">
                          {Object.entries(item.details).map(([k, v]) => (
                            <span key={k} className="whitespace-nowrap">
                              <span className="text-foreground/60">{DETAIL_LABELS[k] ?? k}:</span>{" "}
                              <span className="text-foreground">{formatDetailValue(k, v)}</span>
                            </span>
                          ))}
                        </div>
                      ) : (
                        !item.error && <span>-</span>
                      )}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>

        {shownTaskId && (
          <div className="mt-4 text-xs text-muted-foreground">
            任务 ID: {shownTaskId}
          </div>
        )}
      </CardContent>
    </Card>
  )
}
