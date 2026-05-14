"use client"

import { useState } from "react"
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

interface SLAMonthEntry {
  month: string
  uptime_pct: number
  total_checks: number
  failed_checks: number
}

interface SLAMonitorEntry {
  id: string
  name: string
  type: string
  months: SLAMonthEntry[]
  avg_uptime_pct: number
}

const MOCK_SLA_DATA: SLAMonitorEntry[] = [
  {
    id: "mon-001",
    name: "API 网关健康检查",
    type: "http",
    months: [
      { month: "2026-03", uptime_pct: 99.95, total_checks: 4320, failed_checks: 2 },
      { month: "2026-04", uptime_pct: 100.0, total_checks: 4320, failed_checks: 0 },
      { month: "2026-05", uptime_pct: 99.72, total_checks: 2160, failed_checks: 6 },
    ],
    avg_uptime_pct: 99.89,
  },
  {
    id: "mon-002",
    name: "idcd.com 主站",
    type: "https",
    months: [
      { month: "2026-03", uptime_pct: 100.0, total_checks: 8640, failed_checks: 0 },
      { month: "2026-04", uptime_pct: 99.98, total_checks: 8640, failed_checks: 2 },
      { month: "2026-05", uptime_pct: 99.91, total_checks: 4320, failed_checks: 4 },
    ],
    avg_uptime_pct: 99.96,
  },
  {
    id: "mon-003",
    name: "数据库心跳",
    type: "tcp",
    months: [
      { month: "2026-03", uptime_pct: 98.61, total_checks: 8640, failed_checks: 120 },
      { month: "2026-04", uptime_pct: 99.54, total_checks: 8640, failed_checks: 40 },
      { month: "2026-05", uptime_pct: 100.0, total_checks: 4320, failed_checks: 0 },
    ],
    avg_uptime_pct: 99.38,
  },
  {
    id: "mon-004",
    name: "SSL 证书到期监控",
    type: "ssl_expiry",
    months: [
      { month: "2026-03", uptime_pct: 100.0, total_checks: 1440, failed_checks: 0 },
      { month: "2026-04", uptime_pct: 100.0, total_checks: 1440, failed_checks: 0 },
      { month: "2026-05", uptime_pct: 100.0, total_checks: 720, failed_checks: 0 },
    ],
    avg_uptime_pct: 100.0,
  },
]

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

function filterByMonths(data: SLAMonitorEntry[], months: number): SLAMonitorEntry[] {
  return data.map((m) => ({
    ...m,
    months: m.months.slice(-months),
  }))
}

export default function ReportsPage() {
  const [months, setMonths] = useState("3")

  const filtered = filterByMonths(MOCK_SLA_DATA, parseInt(months, 10))

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
          <Select value={months} onValueChange={setMonths}>
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

        <div className="flex flex-col gap-6">
          {filtered.map((monitor) => (
            <Card key={monitor.id} data-testid={`monitor-card-${monitor.id}`}>
              <CardHeader className="flex flex-row items-center justify-between gap-4 pb-3">
                <div className="flex items-center gap-3">
                  <CardTitle className="text-base font-semibold">
                    {monitor.name}
                  </CardTitle>
                  <Badge variant="secondary" className="font-mono text-xs">
                    {monitor.type}
                  </Badge>
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
      </div>
    </div>
  )
}
