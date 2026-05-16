import type { Metadata } from "next"
import { getTranslations } from "next-intl/server"
import { cookies } from "next/headers"
import { isValidLocale, defaultLocale, type Locale } from "@/i18n/routing"
import { NodesClient } from "./nodes-client"
import type { AdminNode } from "./nodes-client"

export const metadata: Metadata = { title: "Node Health Dashboard — idcd Admin" }

async function getAdminLocale(): Promise<Locale> {
  const cookieStore = await cookies()
  const val = cookieStore.get("locale")?.value ?? ""
  return isValidLocale(val) ? val : defaultLocale
}

async function fetchNodes(): Promise<AdminNode[]> {
  const base = process.env.INTERNAL_API_URL ?? "http://localhost:8080"
  const token = process.env.ADMIN_TOKEN ?? ""
  try {
    const res = await fetch(`${base}/internal/admin/nodes`, {
      headers: { "X-Admin-Token": token },
      cache: "no-store",
    })
    if (!res.ok) return []
    return res.json()
  } catch {
    return []
  }
}

export default async function NodesPage() {
  const locale = await getAdminLocale()
  const t = await getTranslations({ locale, namespace: "admin" })
  const nodes = await fetchNodes()

  return (
    <div>
      <div className="mb-6">
        <h1 className="text-2xl font-bold tracking-tight">{t("nodes.dashboard")}</h1>
        <p className="mt-1 text-sm text-muted-foreground">{t("nodes.dashboardSubtitle")}</p>
      </div>
      <NodesClient nodes={nodes} />
    </div>
  )
}
