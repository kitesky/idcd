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
import { Checkbox } from "@/components/ui/checkbox"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
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
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const [bulkDeleteOpen, setBulkDeleteOpen] = useState(false)
  const [pendingBulkAction, setPendingBulkAction] = useState<
    "pause" | "resume" | "delete" | null
  >(null)

  const total = monitors.length
  const upCount = monitors.filter((m) => m.status === "UP").length
  const downCount = monitors.filter((m) => m.status === "DOWN").length
  const checkingCount = monitors.filter((m) => m.status !== "PAUSED").length

  const allSelected =
    monitors.length > 0 && selectedIds.size === monitors.length

  function toggleSelectAll() {
    if (allSelected) {
      setSelectedIds(new Set())
    } else {
      setSelectedIds(new Set(monitors.map((m) => m.id)))
    }
  }

  function toggleSelect(id: string) {
    setSelectedIds((prev) => {
      const next = new Set(prev)
      if (next.has(id)) {
        next.delete(id)
      } else {
        next.add(id)
      }
      return next
    })
  }

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
  }

  function deleteMonitor(id: string) {
    setMonitors((prev) => prev.filter((m) => m.id !== id))
    setSelectedIds((prev) => {
      const next = new Set(prev)
      next.delete(id)
      return next
    })
  }

  function executeBulkAction(action: "pause" | "resume" | "delete") {
    if (action === "pause") {
      setMonitors((prev) =>
        prev.map((m) =>
          selectedIds.has(m.id) ? { ...m, status: "PAUSED" as MonitorStatus } : m
        )
      )
    } else if (action === "resume") {
      setMonitors((prev) =>
        prev.map((m) =>
          selectedIds.has(m.id) ? { ...m, status: "UP" as MonitorStatus } : m
        )
      )
    } else if (action === "delete") {
      setMonitors((prev) => prev.filter((m) => !selectedIds.has(m.id)))
    }
    setSelectedIds(new Set())
    setPendingBulkAction(null)
  }

  function requestBulkAction(action: "pause" | "resume" | "delete") {
    if (action === "delete") {
      setPendingBulkAction("delete")
      setBulkDeleteOpen(true)
    } else {
      executeBulkAction(action)
    }
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
              <TableHead className="w-10">
                <Checkbox
                  checked={allSelected}
                  onCheckedChange={toggleSelectAll}
                  aria-label="全选"
                />
              </TableHead>
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
                  colSpan={8}
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
                <TableRow
                  key={monitor.id}
                  data-selected={selectedIds.has(monitor.id)}
                >
                  <TableCell>
                    <Checkbox
                      checked={selectedIds.has(monitor.id)}
                      onCheckedChange={() => toggleSelect(monitor.id)}
                      aria-label={`选择 ${monitor.name}`}
                    />
                  </TableCell>
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
                      <DropdownMenu>
                        <DropdownMenuTrigger asChild>
                          <Button
                            variant="ghost"
                            size="icon"
                            className="h-8 w-8"
                            aria-label="更多操作"
                          >
                            <MoreVertical className="h-4 w-4" />
                          </Button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="end">
                          <DropdownMenuItem
                            onClick={() => router.push(`/app/monitors/${monitor.id}`)}
                          >
                            <Activity className="h-4 w-4" />
                            查看详情
                          </DropdownMenuItem>
                          <DropdownMenuItem
                            className="text-destructive focus:text-destructive"
                            onClick={() => deleteMonitor(monitor.id)}
                          >
                            <Trash2 className="h-4 w-4" />
                            删除
                          </DropdownMenuItem>
                        </DropdownMenuContent>
                      </DropdownMenu>
                    </div>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </Card>

      {/* 批量操作浮动栏 */}
      {selectedIds.size > 0 && (
        <div className="fixed bottom-6 left-1/2 -translate-x-1/2 z-50">
          <div className="flex items-center gap-3 rounded-xl border bg-background px-5 py-3 shadow-lg">
            <span className="text-sm font-medium text-muted-foreground">
              已选择 {selectedIds.size} 个监控
            </span>
            <div className="h-4 w-px bg-border" />
            <Button
              variant="outline"
              size="sm"
              onClick={() => requestBulkAction("pause")}
            >
              <Pause className="mr-1.5 h-3.5 w-3.5" />
              暂停
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={() => requestBulkAction("resume")}
            >
              <Play className="mr-1.5 h-3.5 w-3.5" />
              恢复
            </Button>
            <Button
              variant="destructive"
              size="sm"
              onClick={() => requestBulkAction("delete")}
            >
              <Trash2 className="mr-1.5 h-3.5 w-3.5" />
              删除
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setSelectedIds(new Set())}
            >
              取消
            </Button>
          </div>
        </div>
      )}

      {/* 批量删除确认 Dialog */}
      <AlertDialog open={bulkDeleteOpen} onOpenChange={setBulkDeleteOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>确认批量删除</AlertDialogTitle>
            <AlertDialogDescription>
              即将删除 {selectedIds.size} 个监控，此操作不可撤销。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel onClick={() => setPendingBulkAction(null)}>
              取消
            </AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              onClick={() => {
                setBulkDeleteOpen(false)
                if (pendingBulkAction) {
                  executeBulkAction(pendingBulkAction)
                }
              }}
            >
              确认删除
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
