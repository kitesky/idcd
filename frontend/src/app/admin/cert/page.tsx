import type { Metadata } from "next"
import Link from "next/link"
import { cookies } from "next/headers"
import { getT } from "@/i18n/getT"
import { isValidLocale, defaultLocale, type Locale } from "@/i18n/routing"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import {
  listOrders,
  getCAQuota,
  getDNSHealth,
  formatPercent,
  formatRate,
  type AdminCAQuotaResponse,
  type AdminDNSHealthResponse,
  type AdminListOrdersResponse,
} from "./admin-cert-api"

export const metadata: Metadata = { title: "Cert Admin — idcd Admin" }

const API_BASE = process.env.API_BASE_URL ?? "http://localhost:8080"

async function getAdminLocale(): Promise<Locale> {
  const cookieStore = await cookies()
  const val = cookieStore.get("locale")?.value ?? ""
  return isValidLocale(val) ? val : defaultLocale
}

// fetchDashboard issues the three SSR calls in parallel — each helper
// returns null on failure so the page degrades gracefully when cert-svc
// admin endpoints are unreachable.
async function fetchDashboard(): Promise<{
  orders: AdminListOrdersResponse | null
  quota: AdminCAQuotaResponse | null
  dns: AdminDNSHealthResponse | null
}> {
  const [orders, quota, dns] = await Promise.all([
    listOrders({ limit: 5 }, API_BASE),
    getCAQuota(API_BASE),
    getDNSHealth(API_BASE),
  ])
  return { orders, quota, dns }
}

export default async function CertAdminDashboardPage() {
  const locale = await getAdminLocale()
  const t = await getT("admin", locale)
  const { orders, quota, dns } = await fetchDashboard()

  const totalOrders = orders?.orders.length ?? 0
  const switchedCAs = quota?.rows.filter((r) => r.switched).length ?? 0
  const unhealthyProviders = dns?.rows.filter((r) => r.success_rate >= 0 && r.success_rate < 0.9).length ?? 0

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">{t("cert.dashboard.title")}</h1>
        <p className="mt-1 text-sm text-muted-foreground">{t("cert.dashboard.subtitle")}</p>
      </div>

      <nav className="flex flex-wrap gap-3">
        <Link
          href="/admin/cert/orders"
          className="rounded-md border bg-card px-4 py-2 text-sm transition-colors hover:bg-accent"
        >
          {t("cert.nav.orders")}
        </Link>
        <Link
          href="/admin/cert/quota"
          className="rounded-md border bg-card px-4 py-2 text-sm transition-colors hover:bg-accent"
        >
          {t("cert.nav.quota")}
        </Link>
        <Link
          href="/admin/cert/dns-health"
          className="rounded-md border bg-card px-4 py-2 text-sm transition-colors hover:bg-accent"
        >
          {t("cert.nav.dnsHealth")}
        </Link>
        <Link
          href="/admin/cert/abuse"
          className="rounded-md border bg-card px-4 py-2 text-sm transition-colors hover:bg-accent"
        >
          {t("cert.nav.abuse")}
        </Link>
      </nav>

      <div className="grid gap-4 md:grid-cols-3">
        <Card>
          <CardHeader>
            <CardDescription>{t("cert.dashboard.totalOrders")}</CardDescription>
            <CardTitle className="text-3xl font-bold">{totalOrders}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader>
            <CardDescription>{t("cert.dashboard.caQuotaLabel")}</CardDescription>
            <CardTitle className="text-3xl font-bold">
              {switchedCAs}
              <span className="ml-2 align-middle">
                <Badge variant={switchedCAs > 0 ? "destructive" : "secondary"}>
                  {switchedCAs > 0 ? t("cert.quota.switched") : t("cert.quota.normal")}
                </Badge>
              </span>
            </CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader>
            <CardDescription>{t("cert.dashboard.dnsHealthLabel")}</CardDescription>
            <CardTitle className="text-3xl font-bold">
              {unhealthyProviders}
              <span className="ml-2 align-middle">
                <Badge variant={unhealthyProviders > 0 ? "destructive" : "secondary"}>
                  {unhealthyProviders > 0 ? t("cert.quota.switched") : t("cert.quota.normal")}
                </Badge>
              </span>
            </CardTitle>
          </CardHeader>
        </Card>
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>{t("cert.quota.title")}</CardTitle>
            <CardDescription>{t("cert.quota.subtitle", { threshold: 70 })}</CardDescription>
          </CardHeader>
          <CardContent>
            {!quota || quota.rows.length === 0 ? (
              <p className="text-sm text-muted-foreground">{t("cert.dashboard.noData")}</p>
            ) : (
              <ul className="space-y-2">
                {quota.rows.map((r) => (
                  <li key={r.ca} className="flex items-center justify-between text-sm">
                    <span className="font-mono">{r.ca}</span>
                    <span className="flex items-center gap-2">
                      <span className="font-mono">{formatPercent(Math.max(r.per_account_3h, r.per_registered_domain))}</span>
                      {r.switched ? (
                        <Badge variant="destructive">{t("cert.quota.switched")}</Badge>
                      ) : (
                        <Badge variant="secondary">{t("cert.quota.normal")}</Badge>
                      )}
                    </span>
                  </li>
                ))}
              </ul>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>{t("cert.dnsHealth.title")}</CardTitle>
            <CardDescription>{t("cert.dnsHealth.subtitle")}</CardDescription>
          </CardHeader>
          <CardContent>
            {!dns || dns.rows.length === 0 ? (
              <p className="text-sm text-muted-foreground">{t("cert.dashboard.noData")}</p>
            ) : (
              <ul className="space-y-2">
                {dns.rows.map((r) => (
                  <li key={r.provider} className="flex items-center justify-between text-sm">
                    <span className="font-mono">{r.provider}</span>
                    <span className="flex items-center gap-2">
                      <span className="font-mono">{formatRate(r.success_rate, t("cert.dnsHealth.unknown"))}</span>
                      <span className="text-xs text-muted-foreground">({r.samples})</span>
                    </span>
                  </li>
                ))}
              </ul>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
