import type { Metadata } from "next"
import { cookies } from "next/headers"
import { getT } from "@/i18n/getT"
import { isValidLocale, defaultLocale, type Locale } from "@/i18n/routing"
import { getDNSHealth, type AdminDNSHealthResponse } from "../admin-cert-api"
import { DNSHealthClient } from "./dns-health-client"

export const metadata: Metadata = { title: "DNS Health — idcd Admin" }

const API_BASE = process.env.API_BASE_URL ?? "http://localhost:8080"

async function getAdminLocale(): Promise<Locale> {
  const cookieStore = await cookies()
  const val = cookieStore.get("locale")?.value ?? ""
  return isValidLocale(val) ? val : defaultLocale
}

async function fetchInitial(): Promise<AdminDNSHealthResponse> {
  const data = await getDNSHealth(API_BASE)
  return data ?? { rows: [] }
}

export default async function CertAdminDNSHealthPage() {
  const locale = await getAdminLocale()
  const t = await getT("admin", locale)
  const initial = await fetchInitial()
  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">{t("cert.dnsHealth.title")}</h1>
        <p className="mt-1 text-sm text-muted-foreground">{t("cert.dnsHealth.subtitle")}</p>
      </div>
      <DNSHealthClient initial={initial} />
    </div>
  )
}
