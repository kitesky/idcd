import type { Metadata } from "next"
import { Card, CardContent } from "@/components/ui/card"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { OverallBanner } from "@/components/status/overall-banner"
import { ServiceCard } from "@/components/status/service-card"
import { UptimeBar } from "@/components/status/uptime-bar"
import type { ServiceStatus, MonitorHistory } from "@/components/status/types"

export const metadata: Metadata = {
  title: "idcd 平台状态 — idcd",
  description: "idcd 核心服务（api / cert-svc / gateway / aggregator / notifier / web）与节点 Fleet 的实时可用性与 90 天历史可用率",
  alternates: { canonical: "https://idcd.com/status" },
}

// SSR with 60s ISR. The collector writes new data every 5 min anyway —
// caching shorter just multiplies cost for no visible improvement.
export const revalidate = 60

// SSR runs server-side, so we prefer INTERNAL_API_URL (compose-internal hostname,
// e.g. http://api:8000). Fall back to NEXT_PUBLIC_API_URL for local dev where
// only the public origin is set, then localhost as the last-resort default.
const API_BASE =
  process.env.INTERNAL_API_URL ??
  process.env.NEXT_PUBLIC_API_URL ??
  "http://localhost:8080"

// ── API shapes (mirror apps/api/internal/handler/public_status.go) ────────────

interface OverviewResponse {
  overall_status: ServiceStatus
  generated_at: string
  services: ServiceRow[]
  nodes: NodeCountryGroup[]
}
interface ServiceRow {
  key: string
  current_status: ServiceStatus
  uptime_percent: number
  history: DailyBar[]
}
interface DailyBar {
  day: string
  status: ServiceStatus
  uptime_pct: number
  incident_ids?: number[]
}
interface NodeCountryGroup {
  country_code: string
  online_count: number
  total_count: number
  nodes: NodeRow[]
}
interface NodeRow {
  node_id: string
  city?: string
  ip?: string
  status: ServiceStatus
  last_seen_age_s: number
}

interface IncidentsResponse {
  incidents: Incident[]
}
interface Incident {
  id: number
  service_key: string
  started_at: string
  ended_at?: string | null
  severity: "degradation" | "partial_outage" | "outage" | "maintenance"
  title: string
  summary?: string
  related?: string[]
}

// ── Display data ─────────────────────────────────────────────────────────────

// Service keys are short slugs in DB; this gives them a human-friendly card
// title without hard-coding the full list anywhere (unknown keys still show
// as-is, just without the prettier name).
const SERVICE_DISPLAY: Record<string, { name: string; description: string }> = {
  "api":        { name: "API",            description: "用户后台 + Web API 入口" },
  "cert-svc":   { name: "证书服务",        description: "免费证书签发 / 续签 (ACME)" },
  "gateway":    { name: "Agent 网关",      description: "节点 mTLS / 调度入口" },
  "aggregator": { name: "拨测聚合",        description: "Redis Stream → Postgres" },
  "notifier":   { name: "通知服务",        description: "邮件 / 短信 / 钉钉 / 飞书" },
  "web":        { name: "网站",            description: "idcd.com 前端" },
}

// Severity → label + variant mapping for the recent-events list.
const SEVERITY_DISPLAY: Record<Incident["severity"], { label: string; variant: "destructive" | "warning" | "secondary" | "default" }> = {
  outage:         { label: "中断",     variant: "destructive" },
  partial_outage: { label: "部分中断", variant: "destructive" },
  degradation:    { label: "降级",     variant: "warning" },
  maintenance:    { label: "维护",     variant: "secondary" },
}

// ── Helpers ──────────────────────────────────────────────────────────────────

// Convert ISO country code (e.g. "SG") to flag emoji by shifting each
// uppercase letter into the Regional Indicator Symbol range. Returns the
// original string if input isn't a valid 2-letter code.
function flagEmoji(countryCode: string): string {
  const cc = countryCode.toUpperCase()
  if (cc.length !== 2 || !/^[A-Z]{2}$/.test(cc)) return countryCode
  const base = 0x1f1e6 - "A".charCodeAt(0)
  return String.fromCodePoint(base + cc.charCodeAt(0)) + String.fromCodePoint(base + cc.charCodeAt(1))
}

// Country code → friendly display name. Unknown codes fall back to the code
// itself so we never hide nodes due to a missing label.
const COUNTRY_NAME: Record<string, string> = {
  CN: "中国大陆",
  HK: "中国香港",
  TW: "中国台湾",
  SG: "新加坡",
  JP: "日本",
  KR: "韩国",
  US: "美国",
  GB: "英国",
  DE: "德国",
  FR: "法国",
  AU: "澳大利亚",
  CA: "加拿大",
  IN: "印度",
  XX: "未知地区",
}

// Render "5 秒前" / "3 分钟前" / "未上报" given a non-negative age in seconds.
// -1 (the collector's sentinel for never-seen) becomes "未上报".
function ageLabel(s: number): string {
  if (s < 0) return "未上报"
  if (s < 60) return `${s} 秒前`
  if (s < 3600) return `${Math.floor(s / 60)} 分钟前`
  if (s < 86400) return `${Math.floor(s / 3600)} 小时前`
  return `${Math.floor(s / 86400)} 天前`
}

// Convert API DailyBar[] → component MonitorHistory[]. Field rename only.
function toHistory(bars: DailyBar[]): MonitorHistory[] {
  return bars.map(b => ({ date: b.day, status: b.status, uptime: b.uptime_pct }))
}

// ── Data fetching ────────────────────────────────────────────────────────────

async function getOverview(): Promise<OverviewResponse | null> {
  try {
    const res = await fetch(`${API_BASE}/v1/public/status/overview`, { next: { revalidate: 60 } })
    if (!res.ok) return null
    return await res.json()
  } catch {
    return null
  }
}

async function getIncidents(): Promise<Incident[]> {
  try {
    const res = await fetch(`${API_BASE}/v1/public/status/incidents`, { next: { revalidate: 60 } })
    if (!res.ok) return []
    const data: IncidentsResponse = await res.json()
    return data.incidents ?? []
  } catch {
    return []
  }
}

// ── Page ─────────────────────────────────────────────────────────────────────

export default async function StatusPage() {
  const [overview, incidents] = await Promise.all([getOverview(), getIncidents()])

  if (!overview) {
    return (
      <main className="min-h-screen bg-background">
        <div className="container mx-auto max-w-5xl px-4 py-12">
          <h1 className="mb-8 text-3xl font-bold tracking-tight">idcd 平台状态</h1>
          <Alert variant="destructive">
            <AlertDescription>
              暂时无法加载平台状态数据，请稍后刷新页面重试。
            </AlertDescription>
          </Alert>
        </div>
      </main>
    )
  }

  return (
    <main className="min-h-screen bg-background">
      <div className="container mx-auto max-w-5xl px-4 py-12">
        {/* Header — overall banner */}
        <div className="mb-10">
          <OverallBanner status={overview.overall_status} title="idcd 平台状态" />
          <p className="mt-3 text-center text-xs text-muted-foreground">
            数据更新于 {new Date(overview.generated_at).toLocaleString("zh-CN")}
          </p>
        </div>

        {/* Services */}
        <section className="mb-12" data-testid="services-section">
          <h2 className="mb-4 text-lg font-semibold">核心服务</h2>
          {overview.services.length === 0 ? (
            <Card className="px-5 py-8 text-center text-sm text-muted-foreground">
              暂无服务数据 — 采集器还在预热中，几分钟后回来看看。
            </Card>
          ) : (
            <div className="grid gap-4 sm:grid-cols-2">
              {overview.services.map(svc => {
                const display = SERVICE_DISPLAY[svc.key] ?? { name: svc.key, description: "" }
                return (
                  <ServiceCard
                    key={svc.key}
                    name={display.name}
                    description={display.description}
                    status={svc.current_status}
                    uptimePercent={svc.uptime_percent}
                    history={toHistory(svc.history)}
                  />
                )
              })}
            </div>
          )}
        </section>

        {/* Node Fleet */}
        <section className="mb-12" data-testid="nodes-section">
          <div className="mb-4 flex items-center justify-between">
            <h2 className="text-lg font-semibold">节点 Fleet</h2>
            <Button variant="outline" size="sm" disabled title="即将上线">
              切换地图视图（即将上线）
            </Button>
          </div>
          {overview.nodes.length === 0 ? (
            <Card className="px-5 py-8 text-center text-sm text-muted-foreground">
              暂无节点数据。
            </Card>
          ) : (
            <div className="space-y-4">
              {overview.nodes.map(group => {
                const friendly = COUNTRY_NAME[group.country_code] ?? group.country_code
                const ok = group.online_count === group.total_count
                return (
                  <Card key={group.country_code} data-testid={`node-group-${group.country_code}`}>
                    <CardContent className="p-5">
                      <div className="mb-3 flex items-center justify-between">
                        <div className="flex items-center gap-2 text-base font-semibold">
                          <span aria-hidden>{flagEmoji(group.country_code)}</span>
                          <span>{friendly}</span>
                          <span className="text-xs text-muted-foreground">({group.country_code})</span>
                        </div>
                        <Badge variant={ok ? "success" : "warning"}>
                          {group.online_count}/{group.total_count} 在线
                        </Badge>
                      </div>
                      <div className="divide-y divide-border">
                        {group.nodes.map(n => (
                          <div key={n.node_id} className="flex items-center justify-between gap-3 py-2 text-sm">
                            <div className="flex min-w-0 items-center gap-2">
                              <span aria-hidden className={dotClass(n.status)} />
                              <span className="font-mono text-xs">{n.node_id}</span>
                              {n.city ? <span className="text-xs text-muted-foreground">· {n.city}</span> : null}
                              {n.ip ? <span className="hidden font-mono text-xs text-muted-foreground sm:inline">· {n.ip}</span> : null}
                            </div>
                            <span className="text-xs text-muted-foreground">
                              {n.status === "operational" ? "在线" : n.status === "degraded" ? "延迟" : "离线"}
                              {" · "}
                              {ageLabel(n.last_seen_age_s)}
                            </span>
                          </div>
                        ))}
                      </div>
                    </CardContent>
                  </Card>
                )
              })}
            </div>
          )}
        </section>

        {/* Recent Incidents */}
        <section className="mb-12" data-testid="incidents-section">
          <h2 className="mb-4 text-lg font-semibold">最近事件（30 天内）</h2>
          {incidents.length === 0 ? (
            <Card className="px-5 py-8 text-center text-sm text-muted-foreground">
              最近 30 天内无事件记录。
            </Card>
          ) : (
            <div className="space-y-3">
              {incidents.map(inc => {
                const sev = SEVERITY_DISPLAY[inc.severity]
                const display = SERVICE_DISPLAY[inc.service_key] ?? { name: inc.service_key, description: "" }
                return (
                  <Card key={inc.id} data-testid={`incident-${inc.id}`}>
                    <CardContent className="p-5">
                      <div className="mb-2 flex flex-wrap items-center gap-2">
                        <Badge variant={sev.variant}>{sev.label}</Badge>
                        <span className="text-sm font-semibold">{inc.title}</span>
                        <span className="text-xs text-muted-foreground">· {display.name}</span>
                      </div>
                      {inc.summary ? <p className="mb-2 text-sm text-muted-foreground">{inc.summary}</p> : null}
                      <div className="text-xs text-muted-foreground">
                        开始 {new Date(inc.started_at).toLocaleString("zh-CN")}
                        {inc.ended_at ? <> · 恢复 {new Date(inc.ended_at).toLocaleString("zh-CN")}</> : <> · 仍在进行</>}
                      </div>
                    </CardContent>
                  </Card>
                )
              })}
            </div>
          )}
        </section>

        {/* 90-day legend (top-of-page UptimeBar would clutter; we surface
            it as a small legend strip here so users get the color cue
            once for all the ServiceCard bars). */}
        <section className="mb-12">
          <Card className="p-5">
            <UptimeBar
              history={Array.from({ length: 90 }, () => ({
                date: "",
                status: "operational" as ServiceStatus,
                uptime: 100,
              }))}
              label="过去 90 天可用率图例"
              showLegend
            />
          </Card>
        </section>
      </div>
    </main>
  )
}

// Status dot color — kept inline because it's tiny and tightly coupled to
// the node row layout; promoting to a component would obscure intent.
function dotClass(status: ServiceStatus): string {
  const base = "inline-block h-2 w-2 rounded-full"
  const tone: Record<ServiceStatus, string> = {
    operational: "bg-success",
    degraded:    "bg-warning",
    outage:      "bg-destructive",
    maintenance: "bg-info",
  }
  return `${base} ${tone[status]}`
}
