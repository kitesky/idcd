import type { Metadata } from "next"
import { getTranslations } from "next-intl/server"
import { cookies } from "next/headers"
import { isValidLocale, defaultLocale, type Locale } from "@/i18n/routing"
import { UpgradesClient } from "./upgrades-client"

export const metadata: Metadata = { title: "OTA Rollout — idcd Admin" }

export interface UpgradeRollout {
  id: string
  version: string
  download_url: string
  checksum: string
  rollout_pct: number
  status: "active" | "paused" | "completed"
  created_at: string
  updated_at: string
}

const INTERNAL_API_URL = process.env.INTERNAL_API_URL ?? "http://localhost:8080"
const ADMIN_TOKEN = process.env.ADMIN_TOKEN ?? ""

async function getAdminLocale(): Promise<Locale> {
  const cookieStore = await cookies()
  const val = cookieStore.get("locale")?.value ?? ""
  return isValidLocale(val) ? val : defaultLocale
}

async function fetchRollouts(): Promise<UpgradeRollout[]> {
  try {
    const res = await fetch(`${INTERNAL_API_URL}/internal/admin/upgrade-rollouts`, {
      headers: { Authorization: `Bearer ${ADMIN_TOKEN}` },
      cache: "no-store",
    })
    if (!res.ok) return []
    const j = await res.json()
    return j.data ?? []
  } catch {
    return []
  }
}

export default async function UpgradesPage() {
  const locale = await getAdminLocale()
  const t = await getTranslations({ locale, namespace: "admin" })
  const rollouts = await fetchRollouts()
  return (
    <div>
      <div className="mb-6">
        <h1 className="text-2xl font-bold tracking-tight">{t("upgrades.title")}</h1>
        <p className="mt-1 text-sm text-muted-foreground">{t("upgrades.subtitle")}</p>
      </div>
      <UpgradesClient initialRollouts={rollouts} />
    </div>
  )
}
