"use client"

import { useEffect, useState } from "react"
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  CardDescription,
} from "@/components/ui/card"
import { Progress } from "@/components/ui/progress"
import { Badge } from "@/components/ui/badge"
import { Skeleton } from "@/components/ui/skeleton"
import { cn } from "@/components/ui/index"

interface QuotaUsageItem {
  used: number
  limit: number
}

interface QuotaAPIUsageItem {
  used: number
  limit: number
  reset_at: number
}

interface QuotaData {
  plan: string
  monitors: QuotaUsageItem
  channels: QuotaUsageItem
  status_pages: QuotaUsageItem
  api_calls: QuotaAPIUsageItem
  min_interval_s: number
  max_nodes: number
}

const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

function formatResetTime(resetAtUnix: number): string {
  const d = new Date(resetAtUnix * 1000)
  const hh = String(d.getHours()).padStart(2, "0")
  const mm = String(d.getMinutes()).padStart(2, "0")
  return `明天 ${hh}:${mm}`
}

function progressColor(used: number, limit: number): string {
  if (limit === 0) return ""
  const pct = (used / limit) * 100
  if (pct >= 90) return "bg-destructive"
  if (pct >= 70) return "bg-warning"
  return ""
}

// 过去 7 天 API 调用趋势（演示数据）
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

export function UsageClient() {
  const [data, setData] = useState<QuotaData | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    fetch(`${API_BASE}/v1/account/quota`, { credentials: "include" })
      .then((res) => {
        if (!res.ok) throw new Error(`quota fetch failed: ${res.status}`)
        return res.json()
      })
      .then((json) => setData(json.data as QuotaData))
      .catch(() => setData(null))
      .finally(() => setLoading(false))
  }, [])

  return (
    <div className="space-y-8" data-testid="usage-page">
      {/* ── 用量卡片 ── */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2" data-testid="usage-stats">
        {/* API 调用 */}
        <Card data-testid="usage-card-api-calls">
          <CardHeader className="pb-2">
            <div className="flex items-center justify-between">
              <CardTitle className="text-sm font-medium text-muted-foreground">
                API 调用（今日）
              </CardTitle>
              {!loading && data && data.api_calls.limit > 0 &&
                (data.api_calls.used / data.api_calls.limit) >= 0.9 && (
                  <Badge variant="warning" className="text-xs" data-testid="near-limit-badge-api-calls">
                    接近上限
                  </Badge>
                )}
            </div>
            <CardDescription className="text-xs">
              {loading
                ? null
                : data
                ? `${data.plan === "free" ? "Free" : data.plan} 档每日 ${data.api_calls.limit} 次`
                : "Free 档每日 100 次"}
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-2">
            {loading ? (
              <>
                <Skeleton className="h-8 w-24" data-testid="skeleton-api-calls" />
                <Skeleton className="h-2 w-full" />
              </>
            ) : (
              <>
                <div className="flex items-end gap-1">
                  <span className="text-2xl font-bold tabular-nums">
                    {data?.api_calls.used ?? 0}
                  </span>
                  <span className="text-sm text-muted-foreground mb-0.5">
                    / {data?.api_calls.limit ?? 100} 次
                  </span>
                </div>
                <Progress
                  value={
                    data && data.api_calls.limit > 0
                      ? Math.round((data.api_calls.used / data.api_calls.limit) * 100)
                      : 0
                  }
                  className={cn(
                    "h-2",
                    data
                      ? progressColor(data.api_calls.used, data.api_calls.limit)
                        ? `[&>div]:${progressColor(data.api_calls.used, data.api_calls.limit)}`
                        : ""
                      : ""
                  )}
                  data-testid="progress-api-calls"
                />
                {data && (
                  <p className="text-xs text-muted-foreground">
                    重置时间：{formatResetTime(data.api_calls.reset_at)}
                  </p>
                )}
              </>
            )}
          </CardContent>
        </Card>

        {/* 监控数量 */}
        <Card data-testid="usage-card-monitors">
          <CardHeader className="pb-2">
            <div className="flex items-center justify-between">
              <CardTitle className="text-sm font-medium text-muted-foreground">
                监控项
              </CardTitle>
              {!loading && data && data.monitors.limit > 0 &&
                (data.monitors.used / data.monitors.limit) >= 0.9 && (
                  <Badge variant="warning" className="text-xs" data-testid="near-limit-badge-monitors">
                    接近上限
                  </Badge>
                )}
            </div>
            <CardDescription className="text-xs">
              {loading
                ? null
                : data
                ? `${data.plan === "free" ? "Free" : data.plan} 档上限 ${data.monitors.limit} 个`
                : "Free 档上限 3 个"}
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-2">
            {loading ? (
              <>
                <Skeleton className="h-8 w-24" data-testid="skeleton-monitors" />
                <Skeleton className="h-2 w-full" />
              </>
            ) : (
              <>
                <div className="flex items-end gap-1">
                  <span className="text-2xl font-bold tabular-nums">
                    {data?.monitors.used ?? 0}
                  </span>
                  <span className="text-sm text-muted-foreground mb-0.5">
                    / {data?.monitors.limit ?? 3} 个
                  </span>
                </div>
                <Progress
                  value={
                    data && data.monitors.limit > 0
                      ? Math.round((data.monitors.used / data.monitors.limit) * 100)
                      : 0
                  }
                  className="h-2"
                  data-testid="progress-monitors"
                />
              </>
            )}
          </CardContent>
        </Card>

        {/* 状态页 */}
        <Card data-testid="usage-card-status-pages">
          <CardHeader className="pb-2">
            <div className="flex items-center justify-between">
              <CardTitle className="text-sm font-medium text-muted-foreground">
                状态页
              </CardTitle>
            </div>
            <CardDescription className="text-xs">
              {loading
                ? null
                : data
                ? `${data.plan === "free" ? "Free" : data.plan} 档上限 ${data.status_pages.limit} 个`
                : "Free 档上限 1 个"}
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-2">
            {loading ? (
              <>
                <Skeleton className="h-8 w-24" data-testid="skeleton-status-pages" />
                <Skeleton className="h-2 w-full" />
              </>
            ) : (
              <>
                <div className="flex items-end gap-1">
                  <span className="text-2xl font-bold tabular-nums">
                    {data?.status_pages.used ?? 0}
                  </span>
                  <span className="text-sm text-muted-foreground mb-0.5">
                    / {data?.status_pages.limit ?? 1} 个
                  </span>
                </div>
                <Progress
                  value={
                    data && data.status_pages.limit > 0
                      ? Math.round((data.status_pages.used / data.status_pages.limit) * 100)
                      : 0
                  }
                  className="h-2"
                  data-testid="progress-status-pages"
                />
              </>
            )}
          </CardContent>
        </Card>

        {/* 告警通道 */}
        <Card data-testid="usage-card-alert-channels">
          <CardHeader className="pb-2">
            <div className="flex items-center justify-between">
              <CardTitle className="text-sm font-medium text-muted-foreground">
                告警通道
              </CardTitle>
              {!loading && data && data.channels.limit > 0 &&
                (data.channels.used / data.channels.limit) >= 0.9 && (
                  <Badge variant="warning" className="text-xs" data-testid="near-limit-badge-alert-channels">
                    接近上限
                  </Badge>
                )}
            </div>
            <CardDescription className="text-xs">
              {loading
                ? null
                : data
                ? `${data.plan === "free" ? "Free" : data.plan} 档上限 ${data.channels.limit} 个`
                : "Free 档上限 1 个"}
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-2">
            {loading ? (
              <>
                <Skeleton className="h-8 w-24" data-testid="skeleton-alert-channels" />
                <Skeleton className="h-2 w-full" />
              </>
            ) : (
              <>
                <div className="flex items-end gap-1">
                  <span className="text-2xl font-bold tabular-nums">
                    {data?.channels.used ?? 0}
                  </span>
                  <span className="text-sm text-muted-foreground mb-0.5">
                    / {data?.channels.limit ?? 1} 个
                  </span>
                </div>
                <Progress
                  value={
                    data && data.channels.limit > 0
                      ? Math.round((data.channels.used / data.channels.limit) * 100)
                      : 0
                  }
                  className="h-2"
                  data-testid="progress-alert-channels"
                />
              </>
            )}
          </CardContent>
        </Card>
      </div>

      {/* ── API 调用趋势 ── */}
      <Card data-testid="api-trend-card">
        <CardHeader>
          <CardTitle className="text-base">API 调用趋势（过去 7 天）</CardTitle>
          <CardDescription>每日 API 请求次数统计（演示数据）</CardDescription>
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
