import { getTranslations } from "next-intl/server"
import { cookies } from "next/headers"
import { isValidLocale, defaultLocale, type Locale } from "@/i18n/routing"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"

interface Metrics {
  total_users: number
  active_users_7d: number
  total_monitors: number
  active_monitors: number
  total_nodes: number
  online_nodes: number
  subscriptions: { free: number; pro: number; team: number; enterprise: number }
  mrr_estimate_cny: number
}

const INTERNAL_API_URL = process.env.INTERNAL_API_URL ?? "http://localhost:8080"
const ADMIN_TOKEN = process.env.ADMIN_TOKEN ?? ""

async function getAdminLocale(): Promise<Locale> {
  const cookieStore = await cookies()
  const val = cookieStore.get("locale")?.value ?? ""
  return isValidLocale(val) ? val : defaultLocale
}

async function fetchMetrics(): Promise<Metrics | null> {
  try {
    const res = await fetch(`${INTERNAL_API_URL}/internal/admin/metrics`, {
      headers: { "X-Admin-Token": ADMIN_TOKEN },
      cache: "no-store",
    })
    if (!res.ok) return null
    const j = await res.json()
    return j.data ?? null
  } catch {
    return null
  }
}

function StatCard({ title, value, sub }: { title: string; value: string | number; sub?: string }) {
  return (
    <Card>
      <CardHeader className="pb-2">
        <CardTitle className="text-sm font-medium text-muted-foreground">{title}</CardTitle>
      </CardHeader>
      <CardContent>
        <div className="text-3xl font-bold">{value}</div>
        {sub && <p className="mt-1 text-xs text-muted-foreground">{sub}</p>}
      </CardContent>
    </Card>
  )
}

function PlanBar({ label, count, total }: { label: string; count: number; total: number }) {
  const pct = total > 0 ? Math.round((count / total) * 100) : 0
  return (
    <div className="space-y-1">
      <div className="flex justify-between text-sm">
        <span className="text-muted-foreground">{label}</span>
        <span className="font-medium">{count} <span className="text-muted-foreground">({pct}%)</span></span>
      </div>
      <div className="h-2 overflow-hidden rounded-full bg-muted">
        <div className="h-full rounded-full bg-primary transition-all" style={{ width: `${pct}%` }} />
      </div>
    </div>
  )
}

export default async function MetricsPage() {
  const locale = await getAdminLocale()
  const t = await getTranslations({ locale, namespace: "admin" })
  const metrics = await fetchMetrics()

  if (!metrics) return <p className="text-destructive">{t("metrics.loadFailed")}</p>

  const totalSubs =
    metrics.subscriptions.free + metrics.subscriptions.pro +
    metrics.subscriptions.team + metrics.subscriptions.enterprise

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">{t("metrics.title")}</h1>
      <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
        <StatCard
          title={t("metrics.totalUsers")}
          value={metrics.total_users.toLocaleString()}
          sub={t("metrics.activeUsersRecent", { count: metrics.active_users_7d })}
        />
        <StatCard
          title={t("metrics.activeMonitors")}
          value={metrics.active_monitors}
          sub={t("metrics.totalMonitors", { total: metrics.total_monitors })}
        />
        <StatCard
          title={t("metrics.onlineNodes")}
          value={metrics.online_nodes}
          sub={t("metrics.totalNodes", { total: metrics.total_nodes })}
        />
        <StatCard
          title={t("metrics.mrrEstimate")}
          value={`¥${metrics.mrr_estimate_cny.toLocaleString()}`}
          sub={t("metrics.mrrSub")}
        />
      </div>
      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader><CardTitle className="text-base">{t("metrics.subscriptionDist")}</CardTitle></CardHeader>
          <CardContent className="space-y-4">
            <PlanBar label="Free" count={metrics.subscriptions.free} total={totalSubs} />
            <PlanBar label="Pro" count={metrics.subscriptions.pro} total={totalSubs} />
            <PlanBar label="Team" count={metrics.subscriptions.team} total={totalSubs} />
            <PlanBar label="Enterprise" count={metrics.subscriptions.enterprise} total={totalSubs} />
          </CardContent>
        </Card>
        <Card>
          <CardHeader><CardTitle className="text-base">{t("metrics.nodeHealth")}</CardTitle></CardHeader>
          <CardContent className="flex items-center gap-4 pt-2">
            <div className="text-5xl font-bold">
              {metrics.total_nodes > 0 ? Math.round((metrics.online_nodes / metrics.total_nodes) * 100) : 0}
              <span className="text-2xl text-muted-foreground">%</span>
            </div>
            <div className="space-y-1">
              <Badge variant="default">{t("metrics.online", { count: metrics.online_nodes })}</Badge>
              <br />
              <Badge variant="secondary">{t("metrics.offline", { count: metrics.total_nodes - metrics.online_nodes })}</Badge>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
