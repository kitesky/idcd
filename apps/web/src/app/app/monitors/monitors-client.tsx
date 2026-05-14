"use client"

import { useState, useEffect, useCallback } from "react"
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
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Checkbox } from "@/components/ui/checkbox"
import { Skeleton } from "@/components/ui/skeleton"
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
import { apiRequest } from "@/lib/api"
import { type MonitorType, TYPE_LABELS } from "./mock-data"

// ── Backend API types ────────────────────────────────────────────────────────

export type MonitorStatus = "active" | "paused" | "down" | "degraded"

// Frontend-normalised Monitor shape (camelCase).
export interface Monitor {
  id: string
  name: string
  type: MonitorType
  target: string
  /** Normalised frontend status: UP / DOWN / PAUSED / degraded */
  status: "UP" | "DOWN" | "PAUSED" | "degraded"
  uptimePercent: number
  lastCheckedAt: string
  intervalSeconds: number
}

// Raw shape returned by GET /v1/monitors
interface ApiMonitor {
  id: string
  name: string
  type: string
  target: string
  status: string
  uptime_percent: number
  last_checked_at: string
  interval_seconds: number
}

function normaliseStatus(s: string): Monitor["status"] {
  switch (s) {
    case "active":
      return "UP"
    case "down":
      return "DOWN"
    case "paused":
      return "PAUSED"
    case "degraded":
      return "degraded"
    default:
      return "UP"
  }
}

function fromApi(m: ApiMonitor): Monitor {
  return {
    id: m.id,
    name: m.name,
    type: m.type as MonitorType,
    target: m.target,
    status: normaliseStatus(m.status),
    uptimePercent: m.uptime_percent ?? 0,
    lastCheckedAt: m.last_checked_at ?? new Date().toISOString(),
    intervalSeconds: m.interval_seconds ?? 300,
  }
}

// ── UI helpers ────────────────────────────────────────────────────────────────

function statusBadge(status: Monitor["status"]) {
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

// ── Component ─────────────────────────────────────────────────────────────────

export function MonitorsClient() {
  const router = useRouter()
  const [monitors, setMonitors] = useState<Monitor[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const [bulkDeleteOpen, setBulkDeleteOpen] = useState(false)
  const [pendingBulkAction, setPendingBulkAction] = useState<
    "pause" | "resume" | "delete" | null
  >(null)

  // ── Data fetching ───────────────────────────────────────────────────────────

  const fetchMonitors = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const res = await apiRequest<{ data: { monitors: ApiMonitor[]; total: number } }>(
        "/v1/monitors"
      )
      setMonitors((res.data?.monitors ?? []).map(fromApi))
    } catch (err) {
      setError(err instanceof Error ? err.message : "加载失败")
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchMonitors()
  }, [fetchMonitors])

  // ── Derived stats ───────────────────────────────────────────────────────────

  const total = monitors.length
  const upCount = monitors.filter((m) => m.status === "UP").length
  const downCount = monitors.filter((m) => m.status === "DOWN").length
  const checkingCount = monitors.filter((m) => m.status !== "PAUSED").length

  const allSelected =
    monitors.length > 0 && selectedIds.size === monitors.length

  // ── Selection helpers ───────────────────────────────────────────────────────

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

  // ── Individual PATCH pause/resume ───────────────────────────────────────────

  async function togglePause(id: string) {
    const monitor = monitors.find((m) => m.id === id)
    if (!monitor) return
    const newApiStatus = monitor.status === "PAUSED" ? "active" : "paused"
    // Optimistic update
    setMonitors((prev) =>
      prev.map((m) =>
        m.id === id ? { ...m, status: newApiStatus === "paused" ? "PAUSED" : "UP" } : m
      )
    )
    try {
      await apiRequest(`/v1/monitors/${id}`, {
        method: "PATCH",
        body: JSON.stringify({ status: newApiStatus }),
      })
    } catch {
      // Roll back on failure
      await fetchMonitors()
    }
  }

  // ── Individual DELETE ───────────────────────────────────────────────────────

  async function deleteMonitor(id: string) {
    // Optimistic removal
    setMonitors((prev) => prev.filter((m) => m.id !== id))
    setSelectedIds((prev) => {
      const next = new Set(prev)
      next.delete(id)
      return next
    })
    try {
      await apiRequest(`/v1/monitors/${id}`, { method: "DELETE" })
    } catch {
      await fetchMonitors()
    }
  }

  // ── Bulk operations ─────────────────────────────────────────────────────────

  async function executeBulkAction(action: "pause" | "resume" | "delete") {
    const ids = Array.from(selectedIds)
    setSelectedIds(new Set())
    setPendingBulkAction(null)

    // Optimistic UI update
    if (action === "pause") {
      setMonitors((prev) =>
        prev.map((m) => (selectedIds.has(m.id) ? { ...m, status: "PAUSED" as const } : m))
      )
    } else if (action === "resume") {
      setMonitors((prev) =>
        prev.map((m) => (selectedIds.has(m.id) ? { ...m, status: "UP" as const } : m))
      )
    } else {
      setMonitors((prev) => prev.filter((m) => !selectedIds.has(m.id)))
    }

    try {
      await apiRequest("/v1/monitors/bulk", {
        method: "POST",
        body: JSON.stringify({ action, ids }),
      })
    } catch {
      // On error re-fetch to restore consistent state
      await fetchMonitors()
    }
  }

  function requestBulkAction(action: "pause" | "resume" | "delete") {
    if (action === "delete") {
      setPendingBulkAction("delete")
      setBulkDeleteOpen(true)
    } else {
      executeBulkAction(action)
    }
  }

  // ── Render ────────────────────────────────────────────────────────────────

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
            {loading ? (
              <Skeleton className="h-9 w-12" />
            ) : (
              <p className="text-3xl font-bold tabular-nums">{total}</p>
            )}
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
            {loading ? (
              <Skeleton className="h-9 w-12" />
            ) : (
              <p className="text-3xl font-bold tabular-nums text-success">
                {upCount}
              </p>
            )}
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
            {loading ? (
              <Skeleton className="h-9 w-12" />
            ) : (
              <p className="text-3xl font-bold tabular-nums text-destructive">
                {downCount}
              </p>
            )}
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
            {loading ? (
              <Skeleton className="h-9 w-12" />
            ) : (
              <p className="text-3xl font-bold tabular-nums">{checkingCount}</p>
            )}
          </CardContent>
        </Card>
      </div>

      {/* 错误提示 */}
      {error && (
        <Alert variant="destructive">
          <AlertCircle className="h-4 w-4" />
          <AlertTitle>加载失败</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

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
      <Card className="overflow-x-auto">
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
              <TableHead className="hidden md:table-cell">目标</TableHead>
              <TableHead>状态</TableHead>
              <TableHead className="hidden md:table-cell">最后检查</TableHead>
              <TableHead className="hidden md:table-cell">可用率</TableHead>
              <TableHead className="w-24">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {loading ? (
              Array.from({ length: 5 }).map((_, i) => (
                <TableRow key={i}>
                  <TableCell>
                    <Skeleton className="h-4 w-4 rounded" />
                  </TableCell>
                  <TableCell>
                    <Skeleton className="h-4 w-32" />
                  </TableCell>
                  <TableCell>
                    <Skeleton className="h-5 w-16 rounded-full" />
                  </TableCell>
                  <TableCell className="hidden md:table-cell">
                    <Skeleton className="h-4 w-40" />
                  </TableCell>
                  <TableCell>
                    <Skeleton className="h-5 w-12 rounded-full" />
                  </TableCell>
                  <TableCell className="hidden md:table-cell">
                    <Skeleton className="h-4 w-20" />
                  </TableCell>
                  <TableCell className="hidden md:table-cell">
                    <Skeleton className="h-4 w-12" />
                  </TableCell>
                  <TableCell>
                    <Skeleton className="h-8 w-16" />
                  </TableCell>
                </TableRow>
              ))
            ) : monitors.length === 0 ? (
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
                  <TableCell className="hidden md:table-cell max-w-[200px] truncate font-mono text-xs text-muted-foreground">
                    {monitor.target}
                  </TableCell>
                  <TableCell>{statusBadge(monitor.status)}</TableCell>
                  <TableCell className="hidden md:table-cell text-sm text-muted-foreground">
                    {formatRelativeTime(monitor.lastCheckedAt)}
                  </TableCell>
                  <TableCell className="hidden md:table-cell font-mono text-sm">
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
                            onClick={() =>
                              router.push(`/app/monitors/${monitor.id}`)
                            }
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
        <div className="fixed bottom-6 left-1/2 -translate-x-1/2 z-50 w-max max-w-[calc(100vw-2rem)]">
          <div className="flex items-center gap-2 rounded-xl border bg-background px-4 py-3 shadow-lg">
            <span
              className="text-sm font-medium text-muted-foreground whitespace-nowrap"
              data-testid="bulk-selection-count"
            >
              <span className="hidden sm:inline">已选择 </span>
              {selectedIds.size}
              <span className="hidden sm:inline"> 个监控</span>
            </span>
            <div className="h-4 w-px bg-border" />
            <Button
              variant="outline"
              size="sm"
              onClick={() => requestBulkAction("pause")}
            >
              <Pause className="h-3.5 w-3.5" />
              <span className="hidden sm:inline ml-1.5">暂停</span>
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={() => requestBulkAction("resume")}
            >
              <Play className="h-3.5 w-3.5" />
              <span className="hidden sm:inline ml-1.5">恢复</span>
            </Button>
            <Button
              variant="destructive"
              size="sm"
              onClick={() => requestBulkAction("delete")}
            >
              <Trash2 className="h-3.5 w-3.5" />
              <span className="hidden sm:inline ml-1.5">删除</span>
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setSelectedIds(new Set())}
            >
              <span className="hidden sm:inline">取消</span>
              <span className="sm:hidden">✕</span>
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
