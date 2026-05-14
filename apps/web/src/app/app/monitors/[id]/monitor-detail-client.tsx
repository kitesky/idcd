"use client"

import { useState } from "react"
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
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import {
  type Monitor,
  type MonitorStatus,
  TYPE_LABELS,
  generateMockTrendBlocks,
  generateMockCheckResults,
} from "../mock-data"

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
  monitor: Monitor
}

export function MonitorDetailClient({ monitor }: MonitorDetailClientProps) {
  const [currentMonitor, setCurrentMonitor] = useState<Monitor>(monitor)
  const [hoveredBlock, setHoveredBlock] = useState<number | null>(null)

  const trendBlocks = generateMockTrendBlocks(monitor.id)
  const checkResults = generateMockCheckResults(monitor.id)

  function togglePause() {
    setCurrentMonitor((prev) => ({
      ...prev,
      status: prev.status === "PAUSED" ? "UP" : "PAUSED",
    }))
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

      {/* SSE 实时更新占位 */}
      <div className="flex items-center gap-3 rounded-md border border-dashed px-4 py-3">
        <Badge variant="secondary" className="gap-1.5">
          <Radio className="h-3 w-3 animate-pulse" />
          实时更新中
        </Badge>
        <span className="text-xs text-muted-foreground">
          SSE 实时推送将在 S2 上线后启用，当前展示最近检测快照
        </span>
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

      {/* 趋势图：48 块 */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">近 24h 检测趋势</CardTitle>
          <p className="text-xs text-muted-foreground mt-0.5">
            每块代表一次检测（约 30 分钟间隔），绿=正常，红=异常
          </p>
        </CardHeader>
        <CardContent>
          <div className="flex flex-wrap gap-1" data-testid="trend-blocks">
            {trendBlocks.map((block) => (
              <div
                key={block.index}
                data-testid={`trend-block-${block.index}`}
                className={[
                  "relative h-6 w-6 rounded-sm cursor-default transition-transform hover:scale-125",
                  block.status === "UP"
                    ? "bg-success/80 hover:bg-success"
                    : "bg-destructive/80 hover:bg-destructive",
                ].join(" ")}
                onMouseEnter={() => setHoveredBlock(block.index)}
                onMouseLeave={() => setHoveredBlock(null)}
              >
                {hoveredBlock === block.index && (
                  <div className="absolute bottom-full left-1/2 mb-1 -translate-x-1/2 whitespace-nowrap rounded-md bg-popover px-2 py-1 text-xs shadow-md border z-10">
                    <div>{formatDateTime(block.checkedAt)}</div>
                    <div
                      className={
                        block.status === "UP" ? "text-success" : "text-destructive"
                      }
                    >
                      {block.status}{" "}
                      {block.status === "UP" && `${block.latencyMs}ms`}
                    </div>
                  </div>
                )}
              </div>
            ))}
          </div>
        </CardContent>
      </Card>

      {/* 最近 10 次检测结果 */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">最近检测记录</CardTitle>
        </CardHeader>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>时间</TableHead>
              <TableHead>节点</TableHead>
              <TableHead>状态</TableHead>
              <TableHead>延迟</TableHead>
              <TableHead>错误信息</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {checkResults.map((result, i) => (
              <TableRow key={i}>
                <TableCell className="font-mono text-xs text-muted-foreground">
                  {formatDateTime(result.checkedAt)}
                </TableCell>
                <TableCell className="text-sm">{result.nodeRegion}</TableCell>
                <TableCell>
                  {result.status === "UP" ? (
                    <Badge variant="success">UP</Badge>
                  ) : (
                    <Badge variant="destructive">DOWN</Badge>
                  )}
                </TableCell>
                <TableCell className="font-mono text-xs">
                  {result.status === "UP" ? `${result.latencyMs}ms` : "-"}
                </TableCell>
                <TableCell className="text-xs text-muted-foreground">
                  {result.error ?? "-"}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </Card>
    </div>
  )
}
