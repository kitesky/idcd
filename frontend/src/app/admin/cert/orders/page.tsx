import type { Metadata } from "next"
import { cookies } from "next/headers"
import { getT } from "@/i18n/getT"
import { isValidLocale, defaultLocale, type Locale } from "@/i18n/routing"
import { listOrders, type AdminCertOrder } from "../admin-cert-api"
import { OrdersAdminClient } from "./orders-admin-client"

export const metadata: Metadata = { title: "Cert Orders — idcd Admin" }

const API_BASE = process.env.API_BASE_URL ?? "http://localhost:8080"

async function getAdminLocale(): Promise<Locale> {
  const cookieStore = await cookies()
  const val = cookieStore.get("locale")?.value ?? ""
  return isValidLocale(val) ? val : defaultLocale
}

async function fetchInitialOrders(): Promise<AdminCertOrder[]> {
  const data = await listOrders({ limit: 50 }, API_BASE)
  return data?.orders ?? []
}

export default async function CertAdminOrdersPage() {
  const locale = await getAdminLocale()
  const t = await getT("admin", locale)
  const initial = await fetchInitialOrders()
  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">{t("cert.orders.title")}</h1>
        <p className="mt-1 text-sm text-muted-foreground">{t("cert.orders.subtitle")}</p>
      </div>
      <OrdersAdminClient initialOrders={initial} />
    </div>
  )
}
