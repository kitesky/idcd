"use client"

import { useEffect, useState } from "react"
import Link from "next/link"
import {
  Activity,
  AlertTriangle,
  ArrowRight,
  Bell,
  CheckCircle2,
  Globe,
  LayoutDashboard,
  Pin,
  Plus,
  TrendingUp,
} from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Checkbox } from "@/components/ui/checkbox"
import {
  Sheet,
  SheetContent,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { Skeleton } from "@/components/ui/skeleton"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"

interface MonitorSummary {
  total: number
  up: number
  down: number
  paused: number
}

interface DashboardSummary {
  monitors: MonitorSummary
  checks_today: number
  avg_uptime_7d: number
  incidents_open: number
  alerts_fired_7d: number
  status_pages: number
}

interface MonitorItem {
  id: string
  name: string
  status: string
  last_check_at?: string
}

interface DownMonitor {
  id: string
  name: string
  status: string
  last_check_at?: string
}

type AlertEventStatus = "firing" | "resolved" | "acknowledged"

interface AlertEventItem {
  id: string
  monitorName: string
  status: AlertEventStatus
  startedAt: string
  resolvedAt?: string
}

interface StatCardProps {
  title: string
  value: string | number
  icon: React.ReactNode
  badge?: React.ReactNode
  testId?: string
  loading?: boolean
}

function StatCard({ title, value, icon, badge, testId, loading }: StatCardProps) {
  return (
    <Card data-testid={testId}>
      <CardHeader className="pb-2">
        <CardTitle className="flex items-center gap-2 text-sm font-medium text-muted-foreground">
          {icon}
          {title}
        </CardTitle>
      </CardHeader>
      <CardContent className="flex items-center gap-2">
        {loading ? (
          <Skeleton className="h-8 w-16" />
        ) : (
          <>
            <p className="text-3xl font-bold tabular-nums">{value}</p>
            {badge}
          </>
        )}
      </CardContent>
    </Card>
  )
}

function monitorStatusBadge(status: string) {
  switch (status) {
    case "active":
      return <Badge variant="success">在线</Badge>
    case "down":
      return <Badge variant="destructive">离线</Badge>
    case "paused":
      return <Badge variant="secondary">暂停</Badge>
    default:
      return <Badge variant="outline">{status}</Badge>
  }
}

function formatRelative(iso?: string): string {
  if (!iso) return "—"
  const diff = Math.floor((Date.now() - new Date(iso).getTime()) / 1000)
  if (diff < 60) return `${diff}秒前`
  if (diff < 3600) return `${Math.floor(diff / 60)}分钟前`
  if (diff < 86400) return `${Math.floor(diff / 3600)}小时前`
  return `${Math.floor(diff / 86400)}天前`
}

const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

async function apiFetch<T>(path: string): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, { credentials: "include" })
  if (!res.ok) throw new Error(`fetch ${path} failed: ${res.status}`)
  const json = await res.json()
  return json.data as T
}

export default function DashboardPage() {
  const [summary, setSummary] = useState<DashboardSummary | null>(null)
  const [summaryLoading, setSummaryLoading] = useState(true)

  const [pinnedIDs, setPinnedIDs] = useState<string[]>([])
  const [monitors, setMonitors] = useState<MonitorItem[]>([])
  const [sheetOpen, setSheetOpen] = useState(false)
  const [selected, setSelected] = useState<Set<string>>(new Set())

  const [downMonitors, setDownMonitors] = useState<DownMonitor[]>([])
  const [alertEvents, setAlertEvents] = useState<AlertEventItem[]>([])

  useEffect(() => {
    apiFetch<DashboardSummary>("/v1/dashboard/summary")
      .then(setSummary)
      .catch(() => setSummary(null))
      .finally(() => setSummaryLoading(false))

    apiFetch<{ monitor_ids: string[] }>("/v1/dashboard/pins")
      .then((d) => setPinnedIDs(d.monitor_ids ?? []))
      .catch(() => {})

    apiFetch<{ items: MonitorItem[] }>("/v1/monitors")
      .then((d) => setMonitors(d.items ?? []))
      .catch(() => {})

    apiFetch<{ items: DownMonitor[] }>("/v1/monitors?status=DOWN&limit=5")
      .then((d) => setDownMonitors(d.items ?? []))
      .catch(() => {})

    apiFetch<{ events: AlertEventItem[] }>("/v1/alert-events?limit=5")
      .then((d) => setAlertEvents(d.events ?? []))
      .catch(() => {})
  }, [])

  const pinnedMonitors = monitors.filter((m) => pinnedIDs.includes(m.id))

  function openPinSheet() {
    setSelected(new Set(pinnedIDs))
    setSheetOpen(true)
  }

  function toggleSelect(id: string) {
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(id)) {
        next.delete(id)
      } else if (next.size < 6) {
        next.add(id)
      }
      return next
    })
  }

  async function savePins() {
    const ids = Array.from(selected)
    try {
      const res = await fetch(`${API_BASE}/v1/dashboard/pins`, {
        method: "PUT",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ monitor_ids: ids }),
      })
      if (res.ok) {
        setPinnedIDs(ids)
      }
    } catch {}
    setSheetOpen(false)
  }

  const m = summary?.monitors
  const checksToday = summary?.checks_today ?? 0
  const avgUptime7d = summary?.avg_uptime_7d ?? 0
  const incidentsOpen = summary?.incidents_open ?? 0
  const statusPages = summary?.status_pages ?? 0

  return (
    <>
      <div>
        <h1 className="text-2xl font-bold tracking-tight">总览</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          监控健康状态与关键指标一览
        </p>
      </div>

      <div className="space-y-6">
          <div
            className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-6 gap-4"
            data-testid="stat-cards"
          >
            <StatCard
              testId="stat-monitors-total"
              title="监控总数"
              value={m?.total ?? 0}
              icon={<Activity className="h-4 w-4" />}
              loading={summaryLoading}
            />

            <StatCard
              testId="stat-monitors-up"
              title="在线 / 总数"
              value={`${m?.up ?? 0} / ${m?.total ?? 0}`}
              icon={<CheckCircle2 className="h-4 w-4 text-success" />}
              badge={<Badge variant="success">正常</Badge>}
              loading={summaryLoading}
            />

            <StatCard
              testId="stat-checks-today"
              title="今日拨测次数"
              value={checksToday.toLocaleString()}
              icon={<TrendingUp className="h-4 w-4" />}
              loading={summaryLoading}
            />

            <StatCard
              testId="stat-uptime-7d"
              title="7 日平均可用率"
              value={`${avgUptime7d}%`}
              icon={<Activity className="h-4 w-4 text-success" />}
              loading={summaryLoading}
            />

            <StatCard
              testId="stat-incidents-open"
              title="未解决告警"
              value={incidentsOpen}
              icon={<AlertTriangle className="h-4 w-4 text-destructive" />}
              badge={
                incidentsOpen > 0 ? (
                  <Badge variant="destructive">{incidentsOpen}</Badge>
                ) : undefined
              }
              loading={summaryLoading}
            />

            <StatCard
              testId="stat-status-pages"
              title="状态页数量"
              value={statusPages}
              icon={<Globe className="h-4 w-4" />}
              loading={summaryLoading}
            />
          </div>

          <div>
            <h2 className="mb-4 text-lg font-semibold">快捷入口</h2>
            <div
              className="grid grid-cols-1 gap-4 sm:grid-cols-3"
              data-testid="quick-links"
            >
              <Card className="transition-colors hover:bg-muted/50">
                <CardHeader>
                  <CardTitle className="flex items-center gap-2 text-base">
                    <Plus className="h-5 w-5 text-primary" />
                    新建监控
                  </CardTitle>
                  <CardDescription>添加 HTTP、Ping、SSL 或 DNS 监控项目</CardDescription>
                </CardHeader>
                <CardContent>
                  <Button asChild variant="outline" size="sm">
                    <Link href="/app/monitors/new" data-testid="link-new-monitor">
                      立即创建
                      <ArrowRight className="ml-2 h-4 w-4" />
                    </Link>
                  </Button>
                </CardContent>
              </Card>

              <Card className="transition-colors hover:bg-muted/50">
                <CardHeader>
                  <CardTitle className="flex items-center gap-2 text-base">
                    <Bell className="h-5 w-5 text-primary" />
                    查看告警
                  </CardTitle>
                  <CardDescription>管理告警通道、策略和历史事件</CardDescription>
                </CardHeader>
                <CardContent>
                  <Button asChild variant="outline" size="sm">
                    <Link href="/app/alerts" data-testid="link-alerts">
                      前往告警
                      <ArrowRight className="ml-2 h-4 w-4" />
                    </Link>
                  </Button>
                </CardContent>
              </Card>

              <Card className="transition-colors hover:bg-muted/50">
                <CardHeader>
                  <CardTitle className="flex items-center gap-2 text-base">
                    <LayoutDashboard className="h-5 w-5 text-primary" />
                    管理状态页
                  </CardTitle>
                  <CardDescription>发布和配置对外公开的服务状态页面</CardDescription>
                </CardHeader>
                <CardContent>
                  <Button asChild variant="outline" size="sm">
                    <Link href="/app/status-pages" data-testid="link-status-pages">
                      管理状态页
                      <ArrowRight className="ml-2 h-4 w-4" />
                    </Link>
                  </Button>
                </CardContent>
              </Card>
            </div>
          </div>

          {summary && summary.monitors.down > 0 && (
            <div data-testid="down-monitors-section">
              <h2 className="mb-4 text-lg font-semibold">告警中的监控</h2>
              <Card>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>监控名称</TableHead>
                      <TableHead className="w-24">状态</TableHead>
                      <TableHead className="w-36 hidden sm:table-cell">上次检测</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {downMonitors.length === 0 ? (
                      <TableRow>
                        <TableCell colSpan={3} className="text-center text-muted-foreground py-6">
                          加载中…
                        </TableCell>
                      </TableRow>
                    ) : (
                      downMonitors.map((mon) => (
                        <TableRow key={mon.id}>
                          <TableCell>
                            <Link
                              href={`/app/monitors/${mon.id}`}
                              className="font-medium hover:underline"
                            >
                              {mon.name}
                            </Link>
                          </TableCell>
                          <TableCell>
                            <Badge variant="destructive">DOWN</Badge>
                          </TableCell>
                          <TableCell className="text-muted-foreground text-sm hidden sm:table-cell">
                            {formatRelative(mon.last_check_at)}
                          </TableCell>
                        </TableRow>
                      ))
                    )}
                  </TableBody>
                </Table>
              </Card>
            </div>
          )}

          <div data-testid="alert-events-section">
            <div className="mb-4 flex items-center justify-between">
              <h2 className="text-lg font-semibold">近期告警</h2>
              <Button asChild variant="ghost" size="sm">
                <Link href="/app/alerts" className="flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
                  查看全部
                  <ArrowRight className="h-4 w-4" />
                </Link>
              </Button>
            </div>
            {alertEvents.length === 0 ? (
              <Card>
                <CardContent className="py-6 text-center text-sm text-muted-foreground">
                  最近 7 天无告警
                </CardContent>
              </Card>
            ) : (
              <Card>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>监控名称</TableHead>
                      <TableHead className="w-28">状态</TableHead>
                      <TableHead className="w-36 hidden sm:table-cell">触发时间</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {alertEvents.map((evt) => (
                      <TableRow key={evt.id}>
                        <TableCell className="font-medium">{evt.monitorName}</TableCell>
                        <TableCell>
                          {evt.status === "firing" ? (
                            <Badge variant="destructive">FIRING</Badge>
                          ) : evt.status === "resolved" ? (
                            <Badge variant="success">RESOLVED</Badge>
                          ) : (
                            <Badge variant="secondary">ACK</Badge>
                          )}
                        </TableCell>
                        <TableCell className="text-muted-foreground text-sm hidden sm:table-cell">
                          {formatRelative(evt.startedAt)}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </Card>
            )}
          </div>

          <div data-testid="pinned-monitors-section">
            <div className="mb-4 flex items-center justify-between">
              <h2 className="text-lg font-semibold">置顶监控</h2>
              <Button
                variant="outline"
                size="sm"
                onClick={openPinSheet}
                data-testid="open-pin-sheet"
              >
                <Plus className="mr-1 h-4 w-4" />
                添加
              </Button>
            </div>

            {pinnedMonitors.length === 0 ? (
              <Card data-testid="pinned-empty">
                <CardContent className="flex flex-col items-center justify-center py-10 text-muted-foreground">
                  <Pin className="mb-2 h-8 w-8 opacity-30" />
                  <p className="text-sm">暂无置顶监控，点击 + 添加</p>
                </CardContent>
              </Card>
            ) : (
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3" data-testid="pinned-list">
                {pinnedMonitors.map((mon) => (
                  <Card key={mon.id} data-testid={`pinned-card-${mon.id}`}>
                    <CardHeader className="pb-2">
                      <CardTitle className="flex items-center justify-between text-sm font-medium">
                        <span className="truncate">{mon.name}</span>
                        {monitorStatusBadge(mon.status)}
                      </CardTitle>
                    </CardHeader>
                    <CardContent>
                      <p className="text-xs text-muted-foreground">
                        最近检测：{formatRelative(mon.last_check_at)}
                      </p>
                    </CardContent>
                  </Card>
                ))}
              </div>
            )}
          </div>
        </div>

      <Sheet open={sheetOpen} onOpenChange={setSheetOpen}>
        <SheetContent className="flex flex-col gap-0 p-0 overflow-hidden" data-testid="pin-sheet">
          <SheetHeader className="shrink-0 border-b px-6 py-4">
            <SheetTitle>选择置顶监控（最多 6 个）</SheetTitle>
          </SheetHeader>
          <div className="flex-1 overflow-y-auto px-6 py-6 space-y-2">
            {monitors.length === 0 ? (
              <p className="text-sm text-muted-foreground">暂无监控项</p>
            ) : (
              monitors.map((mon) => (
                <div key={mon.id} className="flex items-center gap-3 rounded-md p-2 hover:bg-muted/50">
                  <Checkbox
                    id={`pin-${mon.id}`}
                    checked={selected.has(mon.id)}
                    onCheckedChange={() => toggleSelect(mon.id)}
                    disabled={!selected.has(mon.id) && selected.size >= 6}
                  />
                  <label
                    htmlFor={`pin-${mon.id}`}
                    className="flex flex-1 cursor-pointer items-center justify-between text-sm"
                  >
                    <span>{mon.name}</span>
                    {monitorStatusBadge(mon.status)}
                  </label>
                </div>
              ))
            )}
          </div>
          <SheetFooter className="shrink-0 border-t px-6 py-4 flex-row gap-3 mt-0">
            <Button variant="outline" className="flex-1" onClick={() => setSheetOpen(false)}>
              取消
            </Button>
            <Button className="flex-1" onClick={savePins} data-testid="save-pins">
              保存
            </Button>
          </SheetFooter>
        </SheetContent>
      </Sheet>
    </>
  )
}
