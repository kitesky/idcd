"use client"

import { useState } from "react"
import Link from "next/link"
import { useRouter } from "next/navigation"
import {
  Activity,
  AlertCircle,
  CheckCircle2,
  MoreVertical,
  Pause,
  Play,
  Plus,
  Server,
  Trash2,
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
  type MonitorType,
  TYPE_LABELS,
} from "./mock-data"

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

function typeBadge(type: MonitorType) {
  return <Badge variant="outline">{TYPE_LABELS[type]}</Badge>
}

function formatRelativeTime(iso: string): string {
  const diff = Math.floor((Date.now() - new Date(iso).getTime()) / 1000)
  if (diff < 60) return `${diff}秒前`
  if (diff < 3600) return `${Math.floor(diff / 60)}分钟前`
  if (diff < 86400) return `${Math.floor(diff / 3600)}小时前`
  return `${Math.floor(diff / 86400)}天前`
}

interface MonitorsClientProps {
  initialMonitors: Monitor[]
}

export function MonitorsClient({ initialMonitors }: MonitorsClientProps) {
  const router = useRouter()
  const [monitors, setMonitors] = useState<Monitor[]>(initialMonitors)
  const [openMenuId, setOpenMenuId] = useState<string | null>(null)

  const total = monitors.length
  const upCount = monitors.filter((m) => m.status === "UP").length
  const downCount = monitors.filter((m) => m.status === "DOWN").length
  const checkingCount = monitors.filter((m) => m.status !== "PAUSED").length

  function togglePause(id: string) {
    setMonitors((prev) =>
      prev.map((m) => {
        if (m.id !== id) return m
        return {
          ...m,
          status: m.status === "PAUSED" ? "UP" : "PAUSED",
        } as Monitor
      })
    )
    setOpenMenuId(null)
  }

  function deleteMonitor(id: string) {
    setMonitors((prev) => prev.filter((m) => m.id !== id))
    setOpenMenuId(null)
  }

  return (
    <div className="space-y-6">
      {/* 统计卡片 */}
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="flex items-center gap-2 text-sm font-medium text-muted-foreground">
              <Server className="h-4 w-4" />
              监控总数
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-3xl font-bold tabular-nums">{total}</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="flex items-center gap-2 text-sm font-medium text-muted-foreground">
              <CheckCircle2 className="h-4 w-4 text-success" />
              正常运行
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-3xl font-bold tabular-nums text-success">
              {upCount}
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="flex items-center gap-2 text-sm font-medium text-muted-foreground">
              <AlertCircle className="h-4 w-4 text-destructive" />
              故障中
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-3xl font-bold tabular-nums text-destructive">
              {downCount}
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="flex items-center gap-2 text-sm font-medium text-muted-foreground">
              <Activity className="h-4 w-4" />
              检测中
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-3xl font-bold tabular-nums">{checkingCount}</p>
          </CardContent>
        </Card>
      </div>

      {/* 操作栏 */}
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold">监控项目</h2>
        <Button asChild>
          <Link href="/app/monitors/new">
            <Plus className="mr-2 h-4 w-4" />
            新建监控
          </Link>
        </Button>
      </div>

      {/* 监控列表表格 */}
      <Card>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>名称</TableHead>
              <TableHead>类型</TableHead>
              <TableHead>目标</TableHead>
              <TableHead>状态</TableHead>
              <TableHead>最后检查</TableHead>
              <TableHead>可用率</TableHead>
              <TableHead className="w-24">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {monitors.length === 0 ? (
              <TableRow>
                <TableCell
                  colSpan={7}
                  className="h-32 text-center text-muted-foreground"
                >
                  暂无监控项目，
                  <Link
                    href="/app/monitors/new"
                    className="text-primary underline-offset-4 hover:underline"
                  >
                    立即创建
                  </Link>
                </TableCell>
              </TableRow>
            ) : (
              monitors.map((monitor) => (
                <TableRow key={monitor.id}>
                  <TableCell>
                    <Link
                      href={`/app/monitors/${monitor.id}`}
                      className="font-medium hover:underline underline-offset-4"
                    >
                      {monitor.name}
                    </Link>
                  </TableCell>
                  <TableCell>{typeBadge(monitor.type)}</TableCell>
                  <TableCell className="max-w-[200px] truncate font-mono text-xs text-muted-foreground">
                    {monitor.target}
                  </TableCell>
                  <TableCell>{statusBadge(monitor.status)}</TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {formatRelativeTime(monitor.lastCheckedAt)}
                  </TableCell>
                  <TableCell className="font-mono text-sm">
                    {monitor.uptimePercent.toFixed(1)}%
                  </TableCell>
                  <TableCell>
                    <div className="flex items-center gap-1">
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-8 w-8"
                        onClick={() => togglePause(monitor.id)}
                        title={
                          monitor.status === "PAUSED" ? "恢复检测" : "暂停检测"
                        }
                      >
                        {monitor.status === "PAUSED" ? (
                          <Play className="h-4 w-4" />
                        ) : (
                          <Pause className="h-4 w-4" />
                        )}
                      </Button>
                      <div className="relative">
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-8 w-8"
                          onClick={() =>
                            setOpenMenuId(
                              openMenuId === monitor.id ? null : monitor.id
                            )
                          }
                          aria-label="更多操作"
                        >
                          <MoreVertical className="h-4 w-4" />
                        </Button>
                        {openMenuId === monitor.id && (
                          <div className="absolute right-0 top-8 z-50 min-w-[120px] rounded-md border bg-popover p-1 shadow-md">
                            <button
                              className="flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-sm hover:bg-accent"
                              onClick={() =>
                                router.push(`/app/monitors/${monitor.id}`)
                              }
                            >
                              <Activity className="h-4 w-4" />
                              查看详情
                            </button>
                            <button
                              className="flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-sm text-destructive hover:bg-accent"
                              onClick={() => deleteMonitor(monitor.id)}
                            >
                              <Trash2 className="h-4 w-4" />
                              删除
                            </button>
                          </div>
                        )}
                      </div>
                    </div>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </Card>
    </div>
  )
}
