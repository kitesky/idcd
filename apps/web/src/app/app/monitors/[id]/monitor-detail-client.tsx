"use client"

import { useState, useEffect } from "react"
import Link from "next/link"
import {
  ArrowLeft,
  Edit,
  Pause,
  Play,
  Trash2,
  Wifi,
  Radio,
} from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { type Monitor, type MonitorStatus, TYPE_LABELS } from "../types"
import { apiRequest, API_BASE } from "@/lib/api"

interface LatestCheck {
  monitor_id: string
  node_id: string
  status: "up" | "down" | "degraded"
  latency_ms: number | null
  checked_at: string
  error: string
}

interface CheckBucket {
  bucket_start: string
  total: number
  success: number
  failure: number
  avg_latency_ms: number
  status: "up" | "down" | "degraded" | "empty"
}

function checkStatusBadge(status: "up" | "down" | "degraded") {
  switch (status) {
    case "up":
      return <Badge variant="success">UP</Badge>
    case "down":
      return <Badge variant="destructive">DOWN</Badge>
    case "degraded":
      return <Badge variant="warning">降级</Badge>
  }
}

function relativeTime(iso: string): string {
  const diffMs = Date.now() - new Date(iso).getTime()
  const diffS = Math.floor(diffMs / 1000)
  if (diffS < 10) return "刚刚"
  if (diffS < 60) return `${diffS}秒前`
  const diffM = Math.floor(diffS / 60)
  if (diffM < 60) return `${diffM}分钟前`
  const diffH = Math.floor(diffM / 60)
  return `${diffH}小时前`
}

function statusBadge(status: MonitorStatus) {
  switch (status) {
    case "UP":
      return <Badge variant="success">UP</Badge>
    case "DOWN":
      return <Badge variant="destructive">DOWN</Badge>
    case "PAUSED":
      return <Badge variant="secondary">PAUSED</Badge>
    case "degraded":
      return <Badge variant="warning">降级</Badge>
  }
}

function formatDateTime(iso: string): string {
  return new Date(iso).toLocaleString("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  })
}

interface MonitorDetailClientProps {
  monitor: Monitor | null
  monitorId: string
}

const EMPTY_BUCKETS: CheckBucket[] = Array.from({ length: 48 }, () => ({
  bucket_start: "",
  total: 0,
  success: 0,
  failure: 0,
  avg_latency_ms: 0,
  status: "empty" as const,
}))

export function MonitorDetailClient({ monitor, monitorId }: MonitorDetailClientProps) {
  const [currentMonitor, setCurrentMonitor] = useState<Monitor | null>(monitor)
  const id = monitor?.id ?? monitorId
  const [hoveredBlock, setHoveredBlock] = useState<number | null>(null)
  const [latestCheck, setLatestCheck] = useState<LatestCheck | null>(null)
  const [checkBuckets, setCheckBuckets] = useState<CheckBucket[] | null>(null)
  const [bucketLoading, setBucketLoading] = useState(true)

  useEffect(() => {
    const url = `${API_BASE}/v1/monitors/${id}/stream`
    const es = new EventSource(url, { withCredentials: true })
    es.addEventListener("check", (e: MessageEvent) => {
      try {
        const data = JSON.parse(e.data)
        if (data?.type === "ping") return
        setLatestCheck(data as LatestCheck)
      } catch {
      }
    })
    es.addEventListener("error", () => {
    })
    return () => es.close()
  }, [id])

  useEffect(() => {
    setBucketLoading(true)
    apiRequest<{ data: { buckets: CheckBucket[] } }>(
      `/v1/monitors/${id}/checks?hours=24`
    )
      .then((json) => {
        const buckets: CheckBucket[] = json?.data?.buckets ?? []
        setCheckBuckets(buckets.length > 0 ? buckets : [])
      })
      .catch(() => {
        setCheckBuckets([])
      })
      .finally(() => {
        setBucketLoading(false)
      })
  }, [id])

  function togglePause() {
    setCurrentMonitor((prev) => {
      if (!prev) return null
      return { ...prev, status: prev.status === "PAUSED" ? "UP" : "PAUSED" }
    })
  }

  if (!currentMonitor) {
    return (
      <div className="flex flex-col items-center gap-4 py-16 text-muted-foreground">
        <p>监控项不存在或已被删除</p>
        <Link href="/app/monitors" className="text-sm underline">返回监控列表</Link>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* 顶部：名称 + 状态 + 操作 */}
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-3">
          <Link
            href="/app/monitors"
            className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground transition-colors"
          >
            <ArrowLeft className="h-4 w-4" />
            监控列表
          </Link>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm">
            <Edit className="mr-2 h-4 w-4" />
            编辑
          </Button>
          <Button variant="outline" size="sm" onClick={togglePause}>
            {currentMonitor.status === "PAUSED" ? (
              <>
                <Play className="mr-2 h-4 w-4" />
                恢复
              </>
            ) : (
              <>
                <Pause className="mr-2 h-4 w-4" />
                暂停
              </>
            )}
          </Button>
          <Button variant="outline" size="sm" className="text-destructive hover:text-destructive">
            <Trash2 className="mr-2 h-4 w-4" />
            删除
          </Button>
        </div>
      </div>

      {/* 监控名称 + 状态 */}
      <div>
        <div className="flex items-center gap-3 flex-wrap">
          <h1 className="text-3xl font-bold tracking-tight">
            {currentMonitor.name}
          </h1>
          {statusBadge(currentMonitor.status)}
          <Badge variant="outline">{TYPE_LABELS[currentMonitor.type]}</Badge>
        </div>
        <p className="mt-1 font-mono text-sm text-muted-foreground">
          {currentMonitor.target}
        </p>
      </div>

      <div className="flex items-center gap-3 rounded-md border border-dashed px-4 py-3" data-testid="sse-live-check">
        <Badge variant="secondary" className="gap-1.5">
          <Radio className="h-3 w-3 animate-pulse" />
          实时更新中
        </Badge>
        {latestCheck ? (
          <div className="flex items-center gap-3 text-xs">
            {checkStatusBadge(latestCheck.status)}
            {latestCheck.latency_ms != null && (
              <span className="font-mono text-muted-foreground">
                {latestCheck.latency_ms}ms
              </span>
            )}
            <span className="text-muted-foreground">
              {relativeTime(latestCheck.checked_at)}
            </span>
          </div>
        ) : (
          <span className="text-xs text-muted-foreground">等待最新检测数据…</span>
        )}
      </div>

      {/* 统计卡片行 */}
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              24h 可用率
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p
              className={[
                "text-4xl font-bold tabular-nums",
                currentMonitor.uptimePercent >= 99
                  ? "text-success"
                  : currentMonitor.uptimePercent >= 95
                    ? "text-warning"
                    : "text-destructive",
              ].join(" ")}
            >
              {currentMonitor.uptimePercent.toFixed(1)}%
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              检测频率
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-bold tabular-nums">
              {currentMonitor.intervalSeconds < 60
                ? `${currentMonitor.intervalSeconds}s`
                : `${currentMonitor.intervalSeconds / 60}m`}
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="flex items-center gap-2 text-sm font-medium text-muted-foreground">
              <Wifi className="h-4 w-4" />
              并发节点
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-bold tabular-nums">
              {currentMonitor.concurrentNodes}
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              当前状态
            </CardTitle>
          </CardHeader>
          <CardContent className="pt-3">
            {statusBadge(currentMonitor.status)}
          </CardContent>
        </Card>
      </div>

      {/* 趋势图：最多 48 块 */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">近 24h 检测趋势</CardTitle>
          <p className="text-xs text-muted-foreground mt-0.5">
            每块代表 30 分钟区间，绿=正常，红=异常，橙=降级，灰=无数据
          </p>
        </CardHeader>
        <CardContent>
          {bucketLoading ? (
            <div className="flex flex-wrap gap-1" data-testid="trend-blocks-loading">
              {Array.from({ length: 48 }).map((_, i) => (
                <Skeleton key={i} className="h-6 w-6 rounded-sm" />
              ))}
            </div>
          ) : (
            <div className="flex flex-wrap gap-1" data-testid="trend-blocks">
              {(checkBuckets && checkBuckets.length > 0 ? checkBuckets : EMPTY_BUCKETS).map(
                (bucket, i) => (
                  <div
                    key={i}
                    data-testid={`trend-block-${i}`}
                    className={[
                      "relative h-6 w-6 rounded-sm border cursor-default transition-transform hover:scale-125",
                      bucket.status === "up"
                        ? "bg-success/20 border-success"
                        : bucket.status === "down"
                          ? "bg-destructive/20 border-destructive"
                          : bucket.status === "degraded"
                            ? "bg-warning/20 border-warning"
                            : "bg-muted border-muted",
                    ].join(" ")}
                    onMouseEnter={() => setHoveredBlock(i)}
                    onMouseLeave={() => setHoveredBlock(null)}
                  >
                    {hoveredBlock === i && (
                      <div className="absolute bottom-full left-1/2 mb-1 -translate-x-1/2 whitespace-nowrap rounded-md bg-popover px-2 py-1 text-xs shadow-md border z-10">
                        {bucket.bucket_start ? (
                          <div className="text-muted-foreground">{formatDateTime(bucket.bucket_start)}</div>
                        ) : (
                          <div className="text-muted-foreground">无数据</div>
                        )}
                        {bucket.total > 0 && (
                          <>
                            <div className="text-success">成功 {bucket.success}</div>
                            {bucket.failure > 0 && (
                              <div className="text-destructive">失败 {bucket.failure}</div>
                            )}
                            {bucket.avg_latency_ms > 0 && (
                              <div className="font-mono">{bucket.avg_latency_ms.toFixed(1)}ms</div>
                            )}
                          </>
                        )}
                      </div>
                    )}
                  </div>
                )
              )}
            </div>
          )}
        </CardContent>
      </Card>

      {/* 最近 10 次检测结果（取最新的 10 个非 empty bucket） */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">最近检测记录</CardTitle>
        </CardHeader>
        <div className="overflow-x-auto">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>时间</TableHead>
              <TableHead>状态</TableHead>
              <TableHead>延迟</TableHead>
              <TableHead>成功/失败</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {bucketLoading ? (
              <TableRow>
                <TableCell colSpan={4} className="text-center text-xs text-muted-foreground py-4">
                  加载中…
                </TableCell>
              </TableRow>
            ) : (checkBuckets && checkBuckets.length > 0
                ? [...checkBuckets].reverse().filter((b) => b.status !== "empty").slice(0, 10)
                : []
              ).length === 0 ? (
              <TableRow>
                <TableCell colSpan={4} className="text-center text-xs text-muted-foreground py-4">
                  暂无检测记录
                </TableCell>
              </TableRow>
            ) : (
              [...(checkBuckets ?? [])]
                .reverse()
                .filter((b) => b.status !== "empty")
                .slice(0, 10)
                .map((bucket, i) => (
                  <TableRow key={i}>
                    <TableCell className="font-mono text-xs text-muted-foreground">
                      {bucket.bucket_start ? formatDateTime(bucket.bucket_start) : "-"}
                    </TableCell>
                    <TableCell>
                      {bucket.status === "up" ? (
                        <Badge variant="success">UP</Badge>
                      ) : bucket.status === "down" ? (
                        <Badge variant="destructive">DOWN</Badge>
                      ) : (
                        <Badge variant="warning">降级</Badge>
                      )}
                    </TableCell>
                    <TableCell className="font-mono text-xs">
                      {bucket.avg_latency_ms > 0 ? `${bucket.avg_latency_ms.toFixed(1)}ms` : "-"}
                    </TableCell>
                    <TableCell className="text-xs text-muted-foreground">
                      <span className="text-success">{bucket.success}</span>
                      {" / "}
                      <span className={bucket.failure > 0 ? "text-destructive" : "text-muted-foreground"}>
                        {bucket.failure}
                      </span>
                    </TableCell>
                  </TableRow>
                ))
            )}
          </TableBody>
        </Table>
        </div>
      </Card>
    </div>
  )
}
