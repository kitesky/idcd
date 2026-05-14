"use client"

import { useState, useEffect, useCallback } from "react"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { Skeleton } from "@/components/ui/skeleton"
import { apiRequest } from "@/lib/api"

// ── API types ──────────────────────────────────────────────────────────────

interface SLAEntry {
  monitor_id: string
  monitor_name: string
  uptime_percent: number
  total_checks: number
  failed_checks: number
  period_start: string
  period_end: string
}

// ── Display types (grouped by monitor) ────────────────────────────────────

interface SLAMonthEntry {
  month: string
  uptime_pct: number
  total_checks: number
  failed_checks: number
}

interface SLAMonitorEntry {
  id: string
  name: string
  months: SLAMonthEntry[]
  avg_uptime_pct: number
}

function groupEntries(entries: SLAEntry[]): SLAMonitorEntry[] {
  const map = new Map<string, SLAMonitorEntry>()
  for (const e of entries) {
    if (!map.has(e.monitor_id)) {
      map.set(e.monitor_id, { id: e.monitor_id, name: e.monitor_name, months: [], avg_uptime_pct: 0 })
    }
    const item = map.get(e.monitor_id)!
    const month = e.period_start.slice(0, 7) // "YYYY-MM"
    item.months.push({
      month,
      uptime_pct: e.uptime_percent,
      total_checks: e.total_checks,
      failed_checks: e.failed_checks,
    })
  }
  // Sort months ascending, compute avg
  for (const item of map.values()) {
    item.months.sort((a, b) => a.month.localeCompare(b.month))
    const sum = item.months.reduce((acc, m) => acc + m.uptime_pct, 0)
    item.avg_uptime_pct = item.months.length > 0 ? sum / item.months.length : 0
  }
  return Array.from(map.values())
}

// ── Badge helper ───────────────────────────────────────────────────────────

function uptimeBadge(pct: number) {
  if (pct >= 99.9) {
    return (
      <Badge
        variant="outline"
        className="border-green-500 text-green-600 dark:text-green-400"
        data-testid="badge-success"
      >
        {pct.toFixed(2)}%
      </Badge>
    )
  }
  if (pct >= 99) {
    return (
      <Badge
        variant="outline"
        className="border-yellow-500 text-yellow-600 dark:text-yellow-400"
        data-testid="badge-warning"
      >
        {pct.toFixed(2)}%
      </Badge>
    )
  }
  return (
    <Badge variant="destructive" data-testid="badge-destructive">
      {pct.toFixed(2)}%
    </Badge>
  )
}

// ── Skeleton rows ──────────────────────────────────────────────────────────

function SkeletonCard() {
  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between gap-4 pb-3">
        <Skeleton className="h-5 w-48" />
        <Skeleton className="h-5 w-24" />
      </CardHeader>
      <CardContent>
        <div className="space-y-2">
          {Array.from({ length: 3 }).map((_, i) => (
            <Skeleton key={i} className="h-8 w-full" />
          ))}
        </div>
      </CardContent>
    </Card>
  )
}

// ── Main page ──────────────────────────────────────────────────────────────

export default function ReportsPage() {
  const [months, setMonths] = useState("3")
  const [monitors, setMonitors] = useState<SLAMonitorEntry[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const loadData = useCallback(async (m: string) => {
    setLoading(true)
    setError(null)
    try {
      const res = await apiRequest<{ data: { entries: SLAEntry[] } }>(
        `/v1/reports/sla?months=${m}`,
      )
      const entries = res?.data?.entries ?? []
      setMonitors(groupEntries(entries))
    } catch (err) {
      setError(err instanceof Error ? err.message : "加载 SLA 数据失败")
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    loadData(months)
  }, [months, loadData])

  function handleMonthsChange(val: string) {
    setMonths(val)
  }

  return (
    <div className="min-h-screen bg-background">
      <div className="container mx-auto px-4 py-8">
        <div className="mb-8 flex items-center justify-between gap-4">
          <div>
            <h1 className="text-3xl font-bold tracking-tight">SLA 月报</h1>
            <p className="mt-2 text-muted-foreground">
              查看每个监控在过去几个月的可用率统计
            </p>
          </div>
          <Select value={months} onValueChange={handleMonthsChange}>
            <SelectTrigger className="w-[140px]" data-testid="months-select">
              <SelectValue placeholder="选择月数" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="3">最近 3 个月</SelectItem>
              <SelectItem value="6">最近 6 个月</SelectItem>
              <SelectItem value="12">最近 12 个月</SelectItem>
            </SelectContent>
          </Select>
        </div>

        {error && (
          <Alert variant="destructive" className="mb-6">
            <AlertDescription data-testid="reports-error">{error}</AlertDescription>
          </Alert>
        )}

        {loading ? (
          <div className="flex flex-col gap-6">
            {Array.from({ length: 3 }).map((_, i) => (
              <SkeletonCard key={i} />
            ))}
          </div>
        ) : monitors.length === 0 ? (
          <div
            className="flex flex-col items-center justify-center rounded-lg border border-dashed py-16 text-center"
            data-testid="reports-empty"
          >
            <p className="text-lg font-medium text-muted-foreground">暂无 SLA 数据</p>
            <p className="mt-1 text-sm text-muted-foreground">
              当前时间范围内没有监控报告，请尝试扩大查询范围。
            </p>
          </div>
        ) : (
          <div className="flex flex-col gap-6">
            {monitors.map((monitor) => (
              <Card key={monitor.id} data-testid={`monitor-card-${monitor.id}`}>
                <CardHeader className="flex flex-row items-center justify-between gap-4 pb-3">
                  <div className="flex items-center gap-3">
                    <CardTitle className="text-base font-semibold">
                      {monitor.name}
                    </CardTitle>
                  </div>
                  <div className="flex items-center gap-2 text-sm text-muted-foreground">
                    <span>平均在线率</span>
                    {uptimeBadge(monitor.avg_uptime_pct)}
                  </div>
                </CardHeader>
                <CardContent>
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>月份</TableHead>
                        <TableHead>在线率</TableHead>
                        <TableHead className="text-right">总检查数</TableHead>
                        <TableHead className="text-right">失败次数</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {monitor.months.map((entry) => (
                        <TableRow key={entry.month}>
                          <TableCell className="font-mono text-sm">
                            {entry.month}
                          </TableCell>
                          <TableCell>{uptimeBadge(entry.uptime_pct)}</TableCell>
                          <TableCell className="text-right text-sm tabular-nums">
                            {entry.total_checks.toLocaleString()}
                          </TableCell>
                          <TableCell className="text-right text-sm tabular-nums">
                            {entry.failed_checks}
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </CardContent>
              </Card>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
