"use client"

import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  CardDescription,
} from "@/components/ui/card"
import { Progress } from "@/components/ui/progress"
import { Badge } from "@/components/ui/badge"
import { cn } from "@/components/ui/index"

interface UsageStat {
  id: string
  label: string
  current: number
  max: number | null
  unit: string
  description: string
  warning?: boolean
}

const USAGE_STATS: UsageStat[] = [
  {
    id: "monitors",
    label: "监控项",
    current: 2,
    max: 3,
    unit: "个",
    description: "Free 档上限 3 个",
  },
  {
    id: "api-calls",
    label: "API 调用（今日）",
    current: 47,
    max: 100,
    unit: "次",
    description: "Free 档每日 100 次",
  },
  {
    id: "retention",
    label: "数据保留",
    current: 7,
    max: 7,
    unit: "天",
    description: "Free 档保留 7 天",
    warning: true,
  },
  {
    id: "alert-channels",
    label: "告警通道",
    current: 1,
    max: 1,
    unit: "个",
    description: "Free 档上限 1 个",
    warning: true,
  },
]

// 过去 7 天 API 调用趋势（mock）
const API_TREND = [
  { day: "周一", count: 32 },
  { day: "周二", count: 58 },
  { day: "周三", count: 41 },
  { day: "周四", count: 75 },
  { day: "周五", count: 89 },
  { day: "周六", count: 23 },
  { day: "今天", count: 47 },
]

const MAX_TREND = Math.max(...API_TREND.map((d) => d.count))

function getProgressColor(stat: UsageStat): string {
  if (stat.max === null) return ""
  const pct = stat.max > 0 ? (stat.current / stat.max) * 100 : 0
  if (pct >= 90 || stat.warning) return "bg-destructive"
  if (pct >= 70) return "bg-warning"
  return ""
}

export function UsageClient() {
  return (
    <div className="space-y-8" data-testid="usage-page">
      {/* ── 用量卡片 ── */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2" data-testid="usage-stats">
        {USAGE_STATS.map((stat) => {
          const pct =
            stat.max !== null && stat.max > 0
              ? Math.round((stat.current / stat.max) * 100)
              : 0
          const progressColor = getProgressColor(stat)
          const isNearLimit = pct >= 90 || stat.warning

          return (
            <Card key={stat.id} data-testid={`usage-card-${stat.id}`}>
              <CardHeader className="pb-2">
                <div className="flex items-center justify-between">
                  <CardTitle className="text-sm font-medium text-muted-foreground">
                    {stat.label}
                  </CardTitle>
                  {isNearLimit && (
                    <Badge variant="warning" className="text-xs" data-testid={`near-limit-badge-${stat.id}`}>
                      接近上限
                    </Badge>
                  )}
                </div>
                <CardDescription className="text-xs">
                  {stat.description}
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-2">
                <div className="flex items-end gap-1">
                  <span className="text-2xl font-bold tabular-nums">
                    {stat.current}
                  </span>
                  {stat.max !== null && (
                    <span className="text-sm text-muted-foreground mb-0.5">
                      / {stat.max} {stat.unit}
                    </span>
                  )}
                </div>
                {stat.max !== null && (
                  <Progress
                    value={pct}
                    className={cn(
                      "h-2",
                      progressColor
                        ? `[&>div]:${progressColor}`
                        : ""
                    )}
                    data-testid={`progress-${stat.id}`}
                  />
                )}
              </CardContent>
            </Card>
          )
        })}
      </div>

      {/* ── API 调用趋势 ── */}
      <Card data-testid="api-trend-card">
        <CardHeader>
          <CardTitle className="text-base">API 调用趋势（过去 7 天）</CardTitle>
          <CardDescription>每日 API 请求次数统计</CardDescription>
        </CardHeader>
        <CardContent>
          <div
            className="flex items-end gap-2 h-36"
            data-testid="api-trend-chart"
          >
            {API_TREND.map((d) => {
              const heightPct = MAX_TREND > 0 ? (d.count / MAX_TREND) * 100 : 0
              return (
                <div
                  key={d.day}
                  className="flex flex-1 flex-col items-center gap-1"
                >
                  <span className="text-xs text-muted-foreground tabular-nums">
                    {d.count}
                  </span>
                  <div
                    className="w-full rounded-t-sm bg-primary/70 transition-all"
                    style={{ height: `${heightPct}%` }}
                    title={`${d.day}: ${d.count} 次`}
                    data-testid={`bar-${d.day}`}
                  />
                  <span className="text-xs text-muted-foreground">{d.day}</span>
                </div>
              )
            })}
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
