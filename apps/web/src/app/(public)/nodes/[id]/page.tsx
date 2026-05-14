import type { Metadata } from "next"
import Link from "next/link"
import { ArrowLeft, Activity, Clock, Wifi } from "lucide-react"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { getMockDiagnostics } from "./mock-diagnostics"

interface Props {
  params: Promise<{ id: string }>
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { id } = await params
  return {
    title: `节点诊断 ${id} - idcd`,
    description: `查看节点 ${id} 的延迟分布、健康趋势和诊断数据`,
  }
}

const STATUS_MAP: Record<string, { label: string; variant: "success" | "warning" | "destructive" | "secondary" }> = {
  active: { label: "运行中", variant: "success" },
  degraded: { label: "降级", variant: "warning" },
  inactive: { label: "离线", variant: "destructive" },
  unknown: { label: "未知", variant: "secondary" },
}

const LATENCY_BARS = [
  { key: "p50" as const, label: "P50" },
  { key: "p90" as const, label: "P90" },
  { key: "p95" as const, label: "P95" },
  { key: "p99" as const, label: "P99" },
]

export default async function NodeDetailPage({ params }: Props) {
  const { id } = await params
  const diag = getMockDiagnostics(id)

  const statusInfo = STATUS_MAP[diag.status] ?? STATUS_MAP["unknown"]
  const dist = diag.latency_distribution
  const maxLatency = dist.max || 1
  const trend = diag.health_trend.slice().reverse()

  const maxTrendLatency = Math.max(...trend.map((p) => p.avg_latency), 1)

  const formatDate = (iso: string) => {
    try {
      return new Date(iso).toLocaleString("zh-CN", {
        timeZone: "Asia/Shanghai",
        year: "numeric",
        month: "2-digit",
        day: "2-digit",
        hour: "2-digit",
        minute: "2-digit",
      })
    } catch {
      return iso
    }
  }

  return (
    <div className="min-h-screen bg-background">
      <div className="container mx-auto px-4 py-8 max-w-5xl">
        <div className="mb-6">
          <Link
            href="/nodes"
            className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground transition-colors"
          >
            <ArrowLeft className="h-4 w-4" />
            返回节点列表
          </Link>
        </div>

        <div className="mb-8 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h1 className="text-2xl font-bold tracking-tight font-mono">{diag.name}</h1>
            <p className="mt-1 text-sm text-muted-foreground">
              {diag.location.city} · {diag.location.country} · {diag.location.asn} · {diag.location.isp}
            </p>
          </div>
          <Badge variant={statusInfo.variant} className="w-fit">{statusInfo.label}</Badge>
        </div>

        <div className="grid grid-cols-2 gap-4 sm:grid-cols-4 mb-6">
          <Card>
            <CardHeader className="pb-1">
              <CardTitle className="flex items-center gap-1.5 text-xs font-medium text-muted-foreground">
                <Wifi className="h-3.5 w-3.5" />
                24h 可用率
              </CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-2xl font-bold tabular-nums">{diag.uptime_24h.toFixed(2)}%</p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="pb-1">
              <CardTitle className="flex items-center gap-1.5 text-xs font-medium text-muted-foreground">
                <Activity className="h-3.5 w-3.5" />
                检测次数
              </CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-2xl font-bold tabular-nums">{diag.checks_24h.toLocaleString()}</p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="pb-1">
              <CardTitle className="text-xs font-medium text-muted-foreground">P50 延迟</CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-2xl font-bold tabular-nums">{dist.p50}<span className="text-sm font-normal text-muted-foreground ml-0.5">ms</span></p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="pb-1">
              <CardTitle className="flex items-center gap-1.5 text-xs font-medium text-muted-foreground">
                <Clock className="h-3.5 w-3.5" />
                最后活跃
              </CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-sm font-medium tabular-nums">{formatDate(diag.last_seen)}</p>
            </CardContent>
          </Card>
        </div>

        <div className="grid gap-6 lg:grid-cols-2">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">延迟分布</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="space-y-3" data-testid="latency-distribution">
                {LATENCY_BARS.map(({ key, label }) => {
                  const value = dist[key]
                  const pct = Math.min(100, (value / maxLatency) * 100)
                  return (
                    <div key={key} className="flex items-center gap-3">
                      <span className="w-8 text-xs font-mono text-muted-foreground">{label}</span>
                      <div className="flex-1 h-5 bg-muted rounded overflow-hidden">
                        <div
                          className="h-full bg-primary/80 rounded"
                          style={{ width: `${pct}%` }}
                        />
                      </div>
                      <span className="w-16 text-right text-xs tabular-nums font-mono">{value} ms</span>
                    </div>
                  )
                })}
                <div className="mt-2 flex justify-between text-xs text-muted-foreground pt-2 border-t">
                  <span>最小 {dist.min} ms</span>
                  <span>最大 {dist.max} ms</span>
                </div>
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="text-base">24h 健康趋势</CardTitle>
            </CardHeader>
            <CardContent>
              <div data-testid="health-trend">
                <div className="flex items-end gap-0.5 h-24" aria-label="24小时延迟趋势">
                  {trend.map((pt, i) => {
                    const barH = Math.max(4, (pt.avg_latency / maxTrendLatency) * 96)
                    const isDown = pt.success_rate < 95
                    return (
                      <div
                        key={i}
                        className="flex-1 flex flex-col items-center justify-end"
                        title={`${new Date(pt.hour).toLocaleString("zh-CN", { hour: "2-digit", minute: "2-digit", timeZone: "Asia/Shanghai" })} — 成功率 ${pt.success_rate.toFixed(1)}% · 平均延迟 ${pt.avg_latency} ms`}
                      >
                        <div
                          className={`w-full rounded-sm ${isDown ? "bg-destructive/70" : "bg-primary/60"}`}
                          style={{ height: `${barH}px` }}
                        />
                      </div>
                    )
                  })}
                </div>
                <div className="flex justify-between mt-1 text-xs text-muted-foreground">
                  <span>24h 前</span>
                  <span>现在</span>
                </div>
                <div className="flex gap-4 mt-3 text-xs text-muted-foreground">
                  <span className="flex items-center gap-1">
                    <span className="inline-block w-2.5 h-2.5 rounded-sm bg-primary/60" />
                    正常
                  </span>
                  <span className="flex items-center gap-1">
                    <span className="inline-block w-2.5 h-2.5 rounded-sm bg-destructive/70" />
                    异常
                  </span>
                </div>
              </div>
            </CardContent>
          </Card>
        </div>

        <Card className="mt-6">
          <CardHeader>
            <CardTitle className="text-base">节点信息</CardTitle>
          </CardHeader>
          <CardContent>
            <dl className="grid grid-cols-2 gap-x-8 gap-y-3 sm:grid-cols-4 text-sm">
              <div>
                <dt className="text-muted-foreground">节点 ID</dt>
                <dd className="font-mono mt-0.5">{diag.node_id}</dd>
              </div>
              <div>
                <dt className="text-muted-foreground">国家/地区</dt>
                <dd className="mt-0.5">{diag.location.country}</dd>
              </div>
              <div>
                <dt className="text-muted-foreground">城市</dt>
                <dd className="mt-0.5">{diag.location.city}</dd>
              </div>
              <div>
                <dt className="text-muted-foreground">ASN</dt>
                <dd className="font-mono mt-0.5">{diag.location.asn}</dd>
              </div>
              <div>
                <dt className="text-muted-foreground">运营商</dt>
                <dd className="mt-0.5">{diag.location.isp}</dd>
              </div>
              <div>
                <dt className="text-muted-foreground">状态</dt>
                <dd className="mt-0.5">
                  <Badge variant={statusInfo.variant} className="text-xs">{statusInfo.label}</Badge>
                </dd>
              </div>
              <div>
                <dt className="text-muted-foreground">今日检测</dt>
                <dd className="tabular-nums mt-0.5">{diag.checks_24h.toLocaleString()} 次</dd>
              </div>
              <div>
                <dt className="text-muted-foreground">24h 可用率</dt>
                <dd className="tabular-nums mt-0.5">{diag.uptime_24h.toFixed(2)}%</dd>
              </div>
            </dl>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
