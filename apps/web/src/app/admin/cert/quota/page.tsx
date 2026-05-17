import type { Metadata } from "next"
import { cookies } from "next/headers"
import { getT } from "@/i18n/getT"
import { isValidLocale, defaultLocale, type Locale } from "@/i18n/routing"
import { getCAQuota, type AdminCAQuotaResponse } from "../admin-cert-api"
import { CAQuotaClient } from "./quota-client"

export const metadata: Metadata = { title: "CA Quota — idcd Admin" }

const API_BASE = process.env.API_BASE_URL ?? "http://localhost:8080"

async function getAdminLocale(): Promise<Locale> {
  const cookieStore = await cookies()
  const val = cookieStore.get("locale")?.value ?? ""
  return isValidLocale(val) ? val : defaultLocale
}

async function fetchInitial(): Promise<AdminCAQuotaResponse> {
  const data = await getCAQuota(API_BASE)
  return data ?? { rows: [], switch_threshold: 0.7 }
}

export default async function CertAdminQuotaPage() {
  const locale = await getAdminLocale()
  const t = await getT("admin", locale)
  const initial = await fetchInitial()
  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">{t("cert.quota.title")}</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          {t("cert.quota.subtitle", { threshold: Math.round(initial.switch_threshold * 100) })}
        </p>
      </div>
      <CAQuotaClient initial={initial} />
    </div>
  )
}
