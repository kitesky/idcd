import type { Metadata } from "next"
import { getT } from "@/i18n/getT"
import { cookies } from "next/headers"
import { isValidLocale, defaultLocale, type Locale } from "@/i18n/routing"
import { RefundClient, type RefundFailedPayment } from "./refund-client"

export const metadata: Metadata = { title: "Refund Failed Dashboard — idcd Admin" }

const API_BASE = process.env.API_BASE_URL ?? "http://localhost:8080"

async function getAdminLocale(): Promise<Locale> {
  const cookieStore = await cookies()
  const val = cookieStore.get("locale")?.value ?? ""
  return isValidLocale(val) ? val : defaultLocale
}

async function fetchRefundFailed(): Promise<RefundFailedPayment[]> {
  try {
    const res = await fetch(`${API_BASE}/v1/admin/refund-failed`, { cache: "no-store" })
    if (!res.ok) return []
    const json = await res.json()
    return json?.data?.payments ?? []
  } catch { return [] }
}

export default async function RefundFailedPage() {
  const locale = await getAdminLocale()
  const t = await getT("admin", locale)
  const payments = await fetchRefundFailed()
  return (
    <div>
      <div className="mb-6">
        <h1 className="text-2xl font-bold tracking-tight">{t("refundFailed.title")}</h1>
        <p className="mt-1 text-sm text-muted-foreground">{t("refundFailed.subtitle")}</p>
      </div>
      <RefundClient initialPayments={payments} />
    </div>
  )
}
