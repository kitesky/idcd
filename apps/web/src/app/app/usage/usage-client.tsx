"use client"

import { useEffect, useState } from "react"
import { useTranslations } from "next-intl"
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
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { cn } from "@/components/ui/index"
import { apiRequest } from "@/lib/api"

interface QuotaUsageItem {
  used: number
  limit: number
}

interface QuotaAPIUsageItem {
  used: number
  limit: number
  reset_at: number
}

interface DayCount {
  date: string
  count: number
}

interface QuotaData {
  plan: string
  monitors: QuotaUsageItem
  channels: QuotaUsageItem
  status_pages: QuotaUsageItem
  api_calls: QuotaAPIUsageItem
  api_calls_trend: DayCount[]
  min_interval_s: number
  max_nodes: number
}

interface PointsData {
  balance: number
  total_earned: number
}

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

function formatTrendLabel(date: string, isLast: boolean): string {
  if (isLast) return "今天"
  const d = new Date(date + "T00:00:00Z")
  return ["日", "一", "二", "三", "四", "五", "六"][d.getUTCDay()]!
}

const REDEEM_OPTIONS = [
  { value: "api_calls", label: "1000 次 API 调用额度（500 积分）", points: 500 },
  { value: "monitors", label: "1 个月 Pro 监控（1000 积分）", points: 1000 },
]

interface PointsBalanceCardProps {
  balance: number | null
  loading: boolean
  onRedeemed: () => void
}

function PointsBalanceCard({ balance, loading, onRedeemed }: PointsBalanceCardProps) {
  const [redeemType, setRedeemType] = useState("")
  const [dialogOpen, setDialogOpen] = useState(false)
  const [redeeming, setRedeeming] = useState(false)
  const [redeemError, setRedeemError] = useState<string | null>(null)

  const selected = REDEEM_OPTIONS.find((o) => o.value === redeemType)
  const canRedeem = selected && balance !== null && balance >= selected.points

  async function handleRedeem() {
    if (!redeemType) return
    setRedeeming(true)
    setRedeemError(null)
    try {
      await apiRequest("/v1/account/points/redeem", {
        method: "POST",
        body: JSON.stringify({ reward_type: redeemType, points: selected?.points ?? 0 }),
      })
      setDialogOpen(false)
      setRedeemType("")
      onRedeemed()
    } catch (err) {
      setRedeemError(err instanceof Error ? err.message : "兑换失败")
    } finally {
      setRedeeming(false)
    }
  }

  return (
    <Card data-testid="points-balance-card">
      <CardHeader className="pb-2">
        <div className="flex items-center justify-between">
          <CardTitle className="text-sm font-medium text-muted-foreground">
            积分余额
          </CardTitle>
          <Badge variant="secondary" className="text-xs" data-testid="points-badge">
            社区节点
          </Badge>
        </div>
        <CardDescription className="text-xs">
          贡献节点每次心跳 +1 积分，激活奖励 +200 积分
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        {loading ? (
          <Skeleton className="h-8 w-24" data-testid="skeleton-points" />
        ) : (
          <div className="flex items-end gap-1">
            <span className="text-2xl font-bold tabular-nums" data-testid="points-value">
              {(balance ?? 0).toLocaleString()}
            </span>
            <span className="text-sm text-muted-foreground mb-0.5">pts</span>
          </div>
        )}
        <Dialog open={dialogOpen} onOpenChange={(open) => {
          setDialogOpen(open)
          if (!open) setRedeemError(null)
        }}>
          <DialogTrigger asChild>
            <Button variant="outline" size="sm" data-testid="redeem-button" disabled={loading}>
              兑换积分
            </Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>兑换积分</DialogTitle>
              <DialogDescription>
                当前余额 {(balance ?? 0).toLocaleString()} pts
              </DialogDescription>
            </DialogHeader>
            <div className="space-y-4 py-2">
              <Select value={redeemType} onValueChange={setRedeemType}>
                <SelectTrigger data-testid="redeem-select">
                  <SelectValue placeholder="选择兑换类型" />
                </SelectTrigger>
                <SelectContent>
                  {REDEEM_OPTIONS.map((o) => (
                    <SelectItem key={o.value} value={o.value}>
                      {o.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {redeemError && (
                <Alert variant="destructive" data-testid="redeem-error-alert">
                  <AlertDescription>{redeemError}</AlertDescription>
                </Alert>
              )}
            </div>
            <DialogFooter>
              <Button
                variant="outline"
                onClick={() => setDialogOpen(false)}
                disabled={redeeming}
              >
                取消
              </Button>
              <Button
                disabled={!canRedeem || redeeming}
                onClick={handleRedeem}
                data-testid="confirm-redeem"
              >
                {redeeming ? "兑换中…" : "确认兑换"}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </CardContent>
    </Card>
  )
}

export function UsageClient() {
  const _t = useTranslations("billing")
  const [data, setData] = useState<QuotaData | null>(null)
  const [loading, setLoading] = useState(true)
  const [pointsData, setPointsData] = useState<PointsData | null>(null)
  const [pointsLoading, setPointsLoading] = useState(true)

  function fetchPoints() {
    setPointsLoading(true)
    apiRequest<{ data: PointsData }>("/v1/account/points")
      .then((json) => setPointsData(json.data))
      .catch(() => setPointsData(null))
      .finally(() => setPointsLoading(false))
  }

  useEffect(() => {
    apiRequest<{ data: QuotaData }>("/v1/account/quota")
      .then((json) => setData(json.data))
      .catch(() => setData(null))
      .finally(() => setLoading(false))

    fetchPoints()
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
          <CardDescription>每日 API 请求次数统计</CardDescription>
        </CardHeader>
        <CardContent>
          {loading ? (
            <div className="flex items-end gap-2 h-36" data-testid="api-trend-chart">
              {Array.from({ length: 7 }).map((_, i) => (
                <div key={i} className="flex flex-1 flex-col items-center gap-1">
                  <Skeleton className="w-full h-20" />
                  <Skeleton className="w-4 h-3" />
                </div>
              ))}
            </div>
          ) : (
            <div className="flex items-end gap-2 h-36" data-testid="api-trend-chart">
              {(data?.api_calls_trend ?? []).map((d, i, arr) => {
                const max = Math.max(...arr.map((x) => x.count), 1)
                const heightPct = (d.count / max) * 100
                const label = formatTrendLabel(d.date, i === arr.length - 1)
                return (
                  <div key={d.date} className="flex flex-1 flex-col items-center gap-1">
                    <span className="text-xs text-muted-foreground tabular-nums">
                      {d.count}
                    </span>
                    <div
                      className="w-full rounded-t-sm bg-primary/70 transition-all"
                      style={{ height: `${heightPct}%` }}
                      title={`${d.date}: ${d.count} 次`}
                      data-testid={`bar-${d.date}`}
                    />
                    <span className="text-xs text-muted-foreground">{label}</span>
                  </div>
                )
              })}
            </div>
          )}
        </CardContent>
      </Card>

      {/* ── 积分余额 ── */}
      <PointsBalanceCard balance={pointsData?.balance ?? null} loading={pointsLoading} onRedeemed={fetchPoints} />
    </div>
  )
}
