"use client"

import { useState, useEffect, useCallback } from "react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
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
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Printer, Download } from "lucide-react"
import { apiRequest } from "@/lib/api"

// ── Print styles ───────────────────────────────────────────────────────────

function GlobalPrintStyle() {
  return (
    <style media="print">{`@media print { .no-print { display: none !important; } }`}</style>
  )
}

// ── Export helpers ─────────────────────────────────────────────────────────

function handlePDFExport() {
  window.print()
}

function handleCSVExport(monitors: SLAMonitorEntry[]) {
  const rows: string[] = ["Monitor,Month,Uptime%,Total Checks,Failed Checks"]
  for (const monitor of monitors) {
    for (const m of monitor.months) {
      rows.push(
        [
          `"${monitor.name.replace(/"/g, '""')}"`,
          m.month,
          m.uptime_pct.toFixed(4),
          m.total_checks,
          m.failed_checks,
        ].join(","),
      )
    }
  }
  const csv = rows.join("\n")
  const blob = new Blob([csv], { type: "text/csv;charset=utf-8;" })
  const url = URL.createObjectURL(blob)
  const today = new Date().toISOString().slice(0, 10)
  const a = document.createElement("a")
  a.href = url
  a.download = `sla-report-${today}.csv`
  a.click()
  URL.revokeObjectURL(url)
}

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

// ── Quarter aggregation ────────────────────────────────────────────────────

type Granularity = "monthly" | "quarterly"

interface SLAQuarterEntry {
  month: string  // reused field, stores "YYYY-Qn" in quarterly mode
  uptime_pct: number
  total_checks: number
  failed_checks: number
}

function toQuarterLabel(month: string): string {
  // month is "YYYY-MM"
  const [year, mm] = month.split("-") as [string, string]
  const q = Math.ceil(parseInt(mm, 10) / 3)
  return `${year}-Q${q}`
}

function aggregateToQuarterly(monitor: SLAMonitorEntry): SLAMonitorEntry {
  const qMap = new Map<string, { total: number; failed: number }>()
  for (const m of monitor.months) {
    const label = toQuarterLabel(m.month)
    const existing = qMap.get(label) ?? { total: 0, failed: 0 }
    existing.total += m.total_checks
    existing.failed += m.failed_checks
    qMap.set(label, existing)
  }
  const quarters: SLAQuarterEntry[] = Array.from(qMap.entries())
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([label, { total, failed }]) => ({
      month: label,
      total_checks: total,
      failed_checks: failed,
      uptime_pct: total > 0 ? ((total - failed) / total) * 100 : 100,
    }))
  const sum = quarters.reduce((acc, q) => acc + q.uptime_pct, 0)
  return {
    ...monitor,
    months: quarters,
    avg_uptime_pct: quarters.length > 0 ? sum / quarters.length : 0,
  }
}

// ── Noise analysis types ───────────────────────────────────────────────────

interface NoisyMonitor {
  monitor_id: string
  firings: number
  flaps: number
}

interface NoiseDayEntry {
  date: string
  firings: number
  flaps: number
}

interface NoiseReportResponse {
  period: { from: string; to: string }
  total_firings: number
  total_flaps: number
  noisiest_monitors: NoisyMonitor[]
  daily_trend: NoiseDayEntry[]
}

// ── Noise analysis Tab ─────────────────────────────────────────────────────

function NoiseTab() {
  const [data, setData] = useState<NoiseReportResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [days, setDays] = useState("7")

  const loadNoise = useCallback(async (d: string) => {
    setLoading(true)
    setError(null)
    try {
      const res = await apiRequest<{ data: NoiseReportResponse }>(
        `/v1/reports/alert-noise?days=${d}`,
      )
      setData(res?.data ?? null)
    } catch (err) {
      setError(err instanceof Error ? err.message : "加载噪音数据失败")
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- loadNoise 内部 await 后 setState；days 变化触发
    void loadNoise(days)
  }, [days, loadNoise])

  const noMonitors = !data || data.noisiest_monitors.length === 0
  const noTrend = !data || data.daily_trend.length === 0

  // Clamp trend to the last 7 days regardless of query param for display
  const trendDays = data?.daily_trend.slice(-7) ?? []

  // Max firings for relative bar sizing
  const maxFirings = trendDays.reduce((m, d) => Math.max(m, d.firings), 1)

  return (
    <div className="space-y-6" data-testid="noise-tab">
      {/* Controls */}
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="text-sm text-muted-foreground">
          分析近期告警触发频率，识别频繁抖动的监控项
        </p>
        <Select value={days} onValueChange={setDays}>
          <SelectTrigger className="w-[140px]" data-testid="noise-days-select">
            <SelectValue placeholder="时间范围" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="7">最近 7 天</SelectItem>
            <SelectItem value="14">最近 14 天</SelectItem>
            <SelectItem value="30">最近 30 天</SelectItem>
          </SelectContent>
        </Select>
      </div>

      {error && (
        <Alert variant="destructive" data-testid="noise-error">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      {/* Summary badges */}
      {!loading && data && (
        <div className="flex flex-wrap gap-3">
          <Card className="flex-1 min-w-[140px]">
            <CardContent className="pt-4 pb-4">
              <p className="text-xs text-muted-foreground mb-1">总触发次数</p>
              <p className="text-2xl font-bold tabular-nums">
                {data.total_firings.toLocaleString()}
              </p>
            </CardContent>
          </Card>
          <Card className="flex-1 min-w-[140px]">
            <CardContent className="pt-4 pb-4">
              <p className="text-xs text-muted-foreground mb-1">总抖动次数</p>
              <p className="text-2xl font-bold tabular-nums">
                {data.total_flaps.toLocaleString()}
              </p>
            </CardContent>
          </Card>
          <Card className="flex-1 min-w-[140px]">
            <CardContent className="pt-4 pb-4">
              <p className="text-xs text-muted-foreground mb-1">统计区间</p>
              <p className="text-sm font-medium tabular-nums">
                {data.period.from} ~ {data.period.to}
              </p>
            </CardContent>
          </Card>
        </div>
      )}

      {/* Top 10 noisy monitors table */}
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base font-semibold">噪音最高监控 Top 10</CardTitle>
        </CardHeader>
        <CardContent className="overflow-x-auto p-0">
          {loading ? (
            <div className="space-y-2 p-4">
              {Array.from({ length: 5 }).map((_, i) => (
                <Skeleton key={i} className="h-9 w-full" />
              ))}
            </div>
          ) : noMonitors ? (
            <div
              className="flex flex-col items-center justify-center py-14 text-center"
              data-testid="noise-empty"
            >
              <p className="text-sm font-medium text-muted-foreground">近期无告警噪音</p>
              <p className="mt-1 text-xs text-muted-foreground">
                所选时间范围内没有告警触发记录
              </p>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="pl-4">排名</TableHead>
                  <TableHead>监控 ID</TableHead>
                  <TableHead className="text-right">触发次数</TableHead>
                  <TableHead className="text-right pr-4">抖动次数</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {data!.noisiest_monitors.slice(0, 10).map((m, idx) => (
                  <TableRow key={m.monitor_id} data-testid={`noise-row-${m.monitor_id}`}>
                    <TableCell className="pl-4 text-sm text-muted-foreground">
                      #{idx + 1}
                    </TableCell>
                    <TableCell className="font-mono text-sm">{m.monitor_id}</TableCell>
                    <TableCell className="text-right tabular-nums">
                      <Badge
                        variant={m.firings >= 10 ? "destructive" : "outline"}
                        className={
                          m.firings < 10
                            ? "border-yellow-500 text-yellow-600 dark:text-yellow-400"
                            : undefined
                        }
                      >
                        {m.firings}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-right tabular-nums pr-4 text-sm">
                      {m.flaps}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      {/* Daily trend — 7 mini cards */}
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base font-semibold">最近 7 天每日趋势</CardTitle>
        </CardHeader>
        <CardContent>
          {loading ? (
            <div className="flex gap-2">
              {Array.from({ length: 7 }).map((_, i) => (
                <Skeleton key={i} className="h-20 flex-1" />
              ))}
            </div>
          ) : noTrend ? (
            <p className="py-8 text-center text-sm text-muted-foreground" data-testid="trend-empty">
              暂无每日趋势数据
            </p>
          ) : (
            <div className="flex gap-2 overflow-x-auto pb-1">
              {trendDays.map((entry) => {
                const heightPct = maxFirings > 0 ? Math.round((entry.firings / maxFirings) * 100) : 0
                const isHigh = entry.firings >= 10
                return (
                  <div
                    key={entry.date}
                    className="flex flex-1 min-w-[72px] flex-col items-center gap-1"
                    data-testid={`trend-day-${entry.date}`}
                  >
                    {/* Relative bar */}
                    <div className="w-full h-16 flex items-end rounded overflow-hidden bg-muted">
                      <div
                        className={`w-full rounded transition-all ${isHigh ? "bg-destructive/80" : "bg-primary/60"}`}
                        style={{ height: `${Math.max(heightPct, entry.firings > 0 ? 8 : 0)}%` }}
                      />
                    </div>
                    <p className="text-lg font-bold tabular-nums leading-none">
                      {entry.firings}
                    </p>
                    <p className="text-[10px] text-muted-foreground leading-none">
                      {entry.date.slice(5)}
                    </p>
                  </div>
                )
              })}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

// ── Main page ──────────────────────────────────────────────────────────────

export default function ReportsPage() {
  const [months, setMonths] = useState("3")
  const [granularity, setGranularity] = useState<Granularity>("monthly")
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
    // eslint-disable-next-line react-hooks/set-state-in-effect -- loadData 内部 await 后 setState；months 变化触发
    void loadData(months)
  }, [months, loadData])

  function handleMonthsChange(val: string) {
    setMonths(val)
  }

  const displayMonitors =
    granularity === "quarterly"
      ? monitors.map(aggregateToQuarterly)
      : monitors

  const periodColumnLabel = granularity === "quarterly" ? "季度" : "月份"

  return (
    <div data-testid="reports-page">
      <GlobalPrintStyle />
      <div className="mb-6">
        <h1 className="text-2xl font-bold tracking-tight">报告</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          SLA 月报与告警噪音分析
        </p>
      </div>

      <Tabs defaultValue="sla">
        <TabsList className="mb-6">
          <TabsTrigger value="sla">SLA 月报</TabsTrigger>
          <TabsTrigger value="noise">告警噪音分析</TabsTrigger>
        </TabsList>

        {/* ── SLA Tab ── */}
        <TabsContent value="sla">
          <div className="mb-6 flex flex-wrap items-center justify-end gap-2 no-print">
            <Button
              variant="outline"
              size="sm"
              onClick={handlePDFExport}
              data-testid="pdf-export-btn"
            >
              <Printer className="mr-2 h-4 w-4" />
              导出 PDF
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={() => handleCSVExport(monitors)}
              data-testid="csv-export-btn"
            >
              <Download className="mr-2 h-4 w-4" />
              导出 CSV
            </Button>
            <Select
              value={granularity}
              onValueChange={(v) => setGranularity(v as Granularity)}
            >
              <SelectTrigger
                className="w-[120px]"
                data-testid="granularity-select"
              >
                <SelectValue placeholder="显示维度" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="monthly">月度</SelectItem>
                <SelectItem value="quarterly">季度</SelectItem>
              </SelectContent>
            </Select>
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
          ) : displayMonitors.length === 0 ? (
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
              {displayMonitors.map((monitor) => (
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
                  <CardContent className="overflow-x-auto">
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>{periodColumnLabel}</TableHead>
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
        </TabsContent>

        {/* ── Noise Tab ── */}
        <TabsContent value="noise">
          <NoiseTab />
        </TabsContent>
      </Tabs>
    </div>
  )
}
