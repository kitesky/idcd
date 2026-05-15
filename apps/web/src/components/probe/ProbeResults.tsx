"use client"

import { Card, CardContent, CardHeader, CardTitle, Badge } from "@/components/ui"
import { useProbePolling } from "@/hooks/useProbePolling"
import type { ProbeResult, ProbeTaskResult } from "@/lib/api"

interface ProbeResultsProps {
  result?: ProbeResult | null  // legacy prop mode
  taskId?: string | null       // new polling mode
  loading?: boolean
  error?: string
}

interface DisplayItem {
  node_id: string
  node_name?: string
  success: boolean
  latency_ms?: number
  error?: string
  details?: Record<string, unknown>
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
  loading = false,
  error: externalError,
}: ProbeResultsProps) {
  const { taskResult, isPolling, error: pollError } = useProbePolling(taskId ?? null)

  const isLoading = loading || isPolling
  const error = externalError ?? pollError

  const displayItems = buildDisplayItems(result, taskResult)
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
            <p className="text-xs text-muted-foreground mt-2">等待节点返回结果...</p>
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
          {totalCount > 0 && (
            <Badge variant={successCount === totalCount ? "default" : "secondary"}>
              成功 {successCount} / {totalCount}
            </Badge>
          )}
        </div>
      </CardHeader>
      <CardContent>
        {isPolling && (
          <div className="mb-4">
            <Badge>正在获取结果...</Badge>
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
                      {item.error ?? (item.details ? JSON.stringify(item.details) : "-")}
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
