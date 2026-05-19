"use client"

import { useState, useEffect, useCallback, useRef } from "react"
import Link from "next/link"
import { useRouter } from "next/navigation"
import { useTranslations } from "next-intl"
import { toast } from "sonner"
import {
  Activity,
  AlertCircle,
  CheckCircle2,
  ChevronLeft,
  ChevronRight,
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
import { Input } from "@/components/ui/input"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
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
import { type MonitorType, TYPE_LABELS } from "./types"

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

function statusBadge(status: Monitor["status"], t: ReturnType<typeof useTranslations<"monitors">>) {
  switch (status) {
    case "UP":
      return <Badge variant="success">UP</Badge>
    case "DOWN":
      return <Badge variant="destructive">DOWN</Badge>
    case "PAUSED":
      return <Badge variant="secondary">PAUSED</Badge>
    case "degraded":
      return <Badge variant="warning">{t("status.degraded")}</Badge>
  }
}

function typeBadge(type: MonitorType, t: ReturnType<typeof useTranslations<"monitors">>) {
  return <Badge variant="outline">{t(`type.${type}` as never) || TYPE_LABELS[type]}</Badge>
}

function formatInterval(seconds: number): string {
  if (seconds < 60) return `${seconds}s`
  if (seconds < 3600) return `${seconds / 60}m`
  return `${seconds / 3600}h`
}

function formatRelativeTime(iso: string, t: ReturnType<typeof useTranslations<"monitors">>): string {
  const diff = Math.floor((Date.now() - new Date(iso).getTime()) / 1000)
  if (diff < 60) return t("time.secondsAgo", { count: diff })
  if (diff < 3600) return t("time.minutesAgo", { count: Math.floor(diff / 60) })
  if (diff < 86400) return t("time.hoursAgo", { count: Math.floor(diff / 3600) })
  return t("time.daysAgo", { count: Math.floor(diff / 86400) })
}

// ── Component ─────────────────────────────────────────────────────────────────

const PAGE_SIZE = 20

export function MonitorsClient() {
  const router = useRouter()
  const t = useTranslations("monitors")
  const [monitors, setMonitors] = useState<Monitor[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  // Single-row pending action: pause / resume / delete (all destructive enough
  // to warrant a confirm step). Stores the monitor so the dialog can show name.
  const [pendingRowAction, setPendingRowAction] = useState<
    { action: "pause" | "resume" | "delete"; monitor: Monitor } | null
  >(null)
  // Bulk pending action: pause / resume / delete. Unified so all three branches
  // share one dialog component.
  const [pendingBulkAction, setPendingBulkAction] = useState<
    "pause" | "resume" | "delete" | null
  >(null)
  const [page, setPage] = useState(1)
  const [total, setTotal] = useState(0)
  const [search, setSearch] = useState("")
  const [statusFilter, setStatusFilter] = useState("")
  // Debounce timer ref for search input
  const searchDebounceRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  // ── Data fetching ───────────────────────────────────────────────────────────

  const fetchMonitors = useCallback(async (p: number, q: string, st: string) => {
    setLoading(true)
    setError(null)
    try {
      const params = new URLSearchParams({ page: String(p), limit: String(PAGE_SIZE) })
      if (q) params.set("search", q)
      if (st) params.set("status", st)
      const res = await apiRequest<{ data: { items: ApiMonitor[]; total: number } }>(
        `/v1/monitors?${params.toString()}`
      )
      setMonitors((res.data?.items ?? []).map(fromApi))
      setTotal(res.data?.total ?? 0)
    } catch (err) {
      setError(err instanceof Error ? err.message : t("error.loadFailed"))
    } finally {
      setLoading(false)
    }
  }, [t])

  // Initial load and re-fetch when page/search/status change
  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- fetchMonitors 内部 await 后 setState
    void fetchMonitors(page, search, statusFilter)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [page, statusFilter])

  // Search is debounced separately; statusFilter and page changes trigger immediately above.

  // ── Derived stats ───────────────────────────────────────────────────────────

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
    } catch (err) {
      // Roll back on failure + surface so the user sees why the click "did nothing"
      const msg = err instanceof Error ? err.message : t("error.updateFailed")
      toast.error(msg)
      await fetchMonitors(page, search, statusFilter)
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
    } catch (err) {
      const msg = err instanceof Error ? err.message : t("error.deleteFailed")
      toast.error(msg)
      await fetchMonitors(page, search, statusFilter)
    }
  }

  // ── Bulk operations ─────────────────────────────────────────────────────────

  async function executeBulkAction(action: "pause" | "resume" | "delete") {
    const ids = Array.from(selectedIds)
    setSelectedIds(new Set())
    setPendingBulkAction(null)

    // Optimistic UI update — use the captured `ids` snapshot, not `selectedIds`
    // which was already cleared on line above.
    const idSet = new Set(ids)
    if (action === "pause") {
      setMonitors((prev) =>
        prev.map((m) => (idSet.has(m.id) ? { ...m, status: "PAUSED" as const } : m))
      )
    } else if (action === "resume") {
      setMonitors((prev) =>
        prev.map((m) => (idSet.has(m.id) ? { ...m, status: "UP" as const } : m))
      )
    } else {
      setMonitors((prev) => prev.filter((m) => !idSet.has(m.id)))
    }

    try {
      await apiRequest("/v1/monitors/bulk", {
        method: "POST",
        body: JSON.stringify({ action, ids }),
      })
    } catch (err) {
      const fallback = action === "delete" ? t("error.deleteFailed") : t("error.updateFailed")
      const msg = err instanceof Error ? err.message : fallback
      toast.error(msg)
      await fetchMonitors(page, search, statusFilter)
    }
  }

  // ── Confirm flow ───────────────────────────────────────────────────────────
  // Bulk: affects many monitors at once. Row: pause silently muting alerts for
  // a critical service is a foot-gun. Both stage the action and execute only on
  // explicit confirm. Resume is included for symmetry — same flow regardless of
  // direction.

  function requestRowAction(action: "pause" | "resume" | "delete", monitor: Monitor) {
    setPendingRowAction({ action, monitor })
  }
  function confirmRowAction() {
    if (!pendingRowAction) return
    const { action, monitor } = pendingRowAction
    setPendingRowAction(null)
    if (action === "delete") {
      void deleteMonitor(monitor.id)
    } else {
      void togglePause(monitor.id)
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
              {t("stats.total")}
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
              {t("stats.up")}
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
              {t("stats.down")}
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
              {t("stats.checking")}
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
          <AlertTitle>{t("error.loadFailed")}</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      {/* 操作栏 */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <h2 className="text-lg font-semibold">{t("toolbar.title")}</h2>
        <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
          {/* 搜索框 */}
          <Input
            placeholder={t("toolbar.searchPlaceholder")}
            value={search}
            onChange={(e) => {
              const val = e.target.value
              setSearch(val)
              if (searchDebounceRef.current) clearTimeout(searchDebounceRef.current)
              searchDebounceRef.current = setTimeout(() => {
                setPage(1)
                fetchMonitors(1, val, statusFilter)
              }, 300)
            }}
            className="h-8 w-full sm:w-48"
          />
          {/* 状态筛选 */}
          <Select
            value={statusFilter || "all"}
            onValueChange={(val) => {
              const st = val === "all" ? "" : val
              setStatusFilter(st)
              setPage(1)
            }}
          >
            <SelectTrigger className="h-8 w-full sm:w-32">
              <SelectValue placeholder={t("toolbar.filterAllStatus")} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t("toolbar.filterAll")}</SelectItem>
              <SelectItem value="UP">UP</SelectItem>
              <SelectItem value="DOWN">DOWN</SelectItem>
              <SelectItem value="PAUSED">PAUSED</SelectItem>
            </SelectContent>
          </Select>
          <Button asChild className="h-8">
            <Link href="/app/monitors/new">
              <Plus className="mr-2 h-4 w-4" />
              {t("newMonitor")}
            </Link>
          </Button>
        </div>
      </div>

      {/* 结果计数 */}
      {!loading && (
        <p className="text-sm text-muted-foreground -mb-4">
          {search || statusFilter
            ? t("toolbar.found", { count: total })
            : t("toolbar.totalCount", { count: total })}
        </p>
      )}

      {/* 监控列表表格 */}
      <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-10">
                <Checkbox
                  checked={allSelected}
                  onCheckedChange={toggleSelectAll}
                  aria-label={t("table.selectAll")}
                />
              </TableHead>
              <TableHead>{t("table.name")}</TableHead>
              <TableHead>{t("table.type")}</TableHead>
              <TableHead className="hidden md:table-cell">{t("table.target")}</TableHead>
              <TableHead>{t("table.status")}</TableHead>
              <TableHead className="hidden md:table-cell">{t("table.lastChecked")}</TableHead>
              <TableHead className="hidden md:table-cell">{t("table.uptime")}</TableHead>
              <TableHead className="w-24">{t("table.actions")}</TableHead>
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
                  {t("empty.title")}，
                  <Link
                    href="/app/monitors/new"
                    className="text-primary underline-offset-4 hover:underline"
                  >
                    {t("empty.desc")}
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
                      aria-label={t("bulk.selectRow", { name: monitor.name })}
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
                  <TableCell>
                    <div className="flex items-center gap-1.5">
                      {typeBadge(monitor.type, t)}
                      <span className="text-xs text-muted-foreground hidden sm:inline">
                        {formatInterval(monitor.intervalSeconds)}
                      </span>
                    </div>
                  </TableCell>
                  <TableCell className="hidden md:table-cell max-w-[200px] truncate font-mono text-xs text-muted-foreground">
                    {monitor.target}
                  </TableCell>
                  <TableCell>{statusBadge(monitor.status, t)}</TableCell>
                  <TableCell className="hidden md:table-cell text-sm text-muted-foreground">
                    {formatRelativeTime(monitor.lastCheckedAt, t)}
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
                        onClick={() =>
                          requestRowAction(
                            monitor.status === "PAUSED" ? "resume" : "pause",
                            monitor,
                          )
                        }
                        title={
                          monitor.status === "PAUSED" ? t("actions.resume") : t("actions.pause")
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
                            aria-label={t("actions.moreActions")}
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
                            {t("actions.viewDetail")}
                          </DropdownMenuItem>
                          <DropdownMenuItem
                            className="text-destructive focus:text-destructive"
                            onClick={() => requestRowAction("delete", monitor)}
                          >
                            <Trash2 className="h-4 w-4" />
                            {t("actions.delete")}
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

      {/* 分页 */}
      {total > PAGE_SIZE && (
        <div className="flex items-center justify-between text-sm text-muted-foreground">
          <span>{t("pagination.total", { count: total })}</span>
          <div className="flex items-center gap-1.5">
            <Button
              variant="outline" size="icon" className="h-7 w-7"
              disabled={page <= 1 || loading}
              onClick={() => setPage(p => p - 1)}
            >
              <ChevronLeft className="h-4 w-4" />
            </Button>
            <span className="px-2 tabular-nums">{t("pagination.pageOf", { page, total: Math.ceil(total / PAGE_SIZE) })}</span>
            <Button
              variant="outline" size="icon" className="h-7 w-7"
              disabled={page >= Math.ceil(total / PAGE_SIZE) || loading}
              onClick={() => setPage(p => p + 1)}
            >
              <ChevronRight className="h-4 w-4" />
            </Button>
          </div>
        </div>
      )}

      {/* 批量操作浮动栏 */}
      {selectedIds.size > 0 && (
        <div className="fixed bottom-6 left-1/2 -translate-x-1/2 z-50 w-max max-w-[calc(100vw-2rem)]">
          <div className="flex items-center gap-2 rounded-xl border bg-background px-4 py-3 shadow-lg">
            <span
              className="text-sm font-medium text-muted-foreground whitespace-nowrap"
              data-testid="bulk-selection-count"
            >
              <span className="hidden sm:inline">{t("bulk.selected")} </span>
              {selectedIds.size}
              <span className="hidden sm:inline"> {t("bulk.monitors")}</span>
            </span>
            <div className="h-4 w-px bg-border" />
            <Button
              variant="outline"
              size="sm"
              onClick={() => setPendingBulkAction("pause")}
            >
              <Pause className="h-3.5 w-3.5" />
              <span className="hidden sm:inline ml-1.5">{t("bulk.pause")}</span>
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={() => setPendingBulkAction("resume")}
            >
              <Play className="h-3.5 w-3.5" />
              <span className="hidden sm:inline ml-1.5">{t("bulk.resume")}</span>
            </Button>
            <Button
              variant="destructive"
              size="sm"
              onClick={() => setPendingBulkAction("delete")}
            >
              <Trash2 className="h-3.5 w-3.5" />
              <span className="hidden sm:inline ml-1.5">{t("bulk.delete")}</span>
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setSelectedIds(new Set())}
            >
              <span className="hidden sm:inline">{t("bulk.cancel")}</span>
              <span className="sm:hidden">✕</span>
            </Button>
          </div>
        </div>
      )}

      {/* 批量操作确认 Dialog — pause/resume/delete 共用，文案按 pendingBulkAction 切换 */}
      <AlertDialog
        open={pendingBulkAction !== null}
        onOpenChange={(open) => {
          if (!open) setPendingBulkAction(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {pendingBulkAction === "delete" && t("confirm.bulkDeleteTitle")}
              {pendingBulkAction === "pause" && t("confirm.bulkPauseTitle")}
              {pendingBulkAction === "resume" && t("confirm.bulkResumeTitle")}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {pendingBulkAction === "delete" &&
                t("confirm.bulkDeleteDesc", { count: selectedIds.size })}
              {pendingBulkAction === "pause" &&
                t("confirm.bulkPauseDesc", { count: selectedIds.size })}
              {pendingBulkAction === "resume" &&
                t("confirm.bulkResumeDesc", { count: selectedIds.size })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("bulk.cancel")}</AlertDialogCancel>
            <AlertDialogAction
              className={
                pendingBulkAction === "delete"
                  ? "bg-destructive text-destructive-foreground hover:bg-destructive/90"
                  : undefined
              }
              onClick={() => {
                const action = pendingBulkAction
                if (action) executeBulkAction(action)
              }}
            >
              {pendingBulkAction === "delete" && t("confirm.confirmDelete")}
              {pendingBulkAction === "pause" && t("confirm.confirmBulkPause")}
              {pendingBulkAction === "resume" && t("confirm.confirmBulkResume")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* 单行操作确认 Dialog — pause/resume/delete 共用 */}
      <AlertDialog
        open={pendingRowAction !== null}
        onOpenChange={(open) => {
          if (!open) setPendingRowAction(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {pendingRowAction?.action === "delete" && t("confirm.deleteTitle")}
              {pendingRowAction?.action === "pause" && t("confirm.pauseTitle")}
              {pendingRowAction?.action === "resume" && t("confirm.resumeTitle")}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {pendingRowAction?.action === "delete" &&
                t("confirm.deleteDesc", { name: pendingRowAction.monitor.name })}
              {pendingRowAction?.action === "pause" &&
                t("confirm.pauseDesc", { name: pendingRowAction.monitor.name })}
              {pendingRowAction?.action === "resume" &&
                t("confirm.resumeDesc", { name: pendingRowAction.monitor.name })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("bulk.cancel")}</AlertDialogCancel>
            <AlertDialogAction
              className={
                pendingRowAction?.action === "delete"
                  ? "bg-destructive text-destructive-foreground hover:bg-destructive/90"
                  : undefined
              }
              onClick={confirmRowAction}
            >
              {pendingRowAction?.action === "delete" && t("confirm.confirmDelete")}
              {pendingRowAction?.action === "pause" && t("confirm.confirmPause")}
              {pendingRowAction?.action === "resume" && t("confirm.confirmResume")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
