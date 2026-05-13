"use client"

import { Card, CardContent, CardHeader, CardTitle, Badge } from "@idcd/ui"
import type { ProbeResult } from "@/lib/api"

interface ProbeResultsProps {
  result: ProbeResult | null
  loading?: boolean
  error?: string
}

export default function ProbeResults({ result, loading = false, error }: ProbeResultsProps) {
  if (loading) {
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

  if (!result) {
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

  const results = result.results || []
  const successCount = results.filter(r => r.success).length
  const totalCount = results.length

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle>拨测结果</CardTitle>
          <Badge variant={successCount === totalCount ? "default" : "secondary"}>
            成功 {successCount} / {totalCount}
          </Badge>
        </div>
      </CardHeader>
      <CardContent>
        {result.status === "pending" && (
          <div className="mb-4">
            <Badge>任务进行中...</Badge>
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
              {results.length === 0 ? (
                <tr>
                  <td colSpan={4} className="text-center py-6 text-muted-foreground">
                    暂无结果
                  </td>
                </tr>
              ) : (
                results.map((item, index) => (
                  <tr key={index} className="border-b">
                    <td className="py-3 px-3">
                      <div className="font-medium">{item.node_name || item.node_id}</div>
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
                      {item.error || (item.details ? JSON.stringify(item.details) : "-")}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>

        {result.task_id && (
          <div className="mt-4 text-xs text-muted-foreground">
            任务 ID: {result.task_id}
          </div>
        )}
      </CardContent>
    </Card>
  )
}
