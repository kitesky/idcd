import type { Metadata } from "next"
import { getTranslations } from "next-intl/server"
import { cookies } from "next/headers"
import { isValidLocale, defaultLocale, type Locale } from "@/i18n/routing"
import { ApplicationsClient, type NodeApplication } from "./applications-client"

export const metadata: Metadata = { title: "Node Application Review — idcd Admin" }

async function getAdminLocale(): Promise<Locale> {
  const cookieStore = await cookies()
  const val = cookieStore.get("locale")?.value ?? ""
  return isValidLocale(val) ? val : defaultLocale
}

async function fetchApplications(): Promise<NodeApplication[]> {
  const base = process.env.INTERNAL_API_URL ?? "http://localhost:8080"
  const token = process.env.ADMIN_TOKEN ?? ""
  try {
    const res = await fetch(`${base}/v1/admin/node-applications`, {
      headers: { Authorization: `Bearer ${token}` },
      cache: "no-store",
    })
    if (!res.ok) return []
    const json = await res.json()
    return (json?.applications ?? []) as NodeApplication[]
  } catch {
    return []
  }
}

export default async function NodeApplicationsPage() {
  const locale = await getAdminLocale()
  const t = await getTranslations({ locale, namespace: "admin" })
  const apps = await fetchApplications()

  return (
    <div>
      <div className="mb-6">
        <h1 className="text-2xl font-bold tracking-tight">{t("nodeApplications.title")}</h1>
        <p className="mt-1 text-sm text-muted-foreground">{t("nodeApplications.subtitle")}</p>
      </div>
      <ApplicationsClient initialApps={apps} />
    </div>
  )
}
