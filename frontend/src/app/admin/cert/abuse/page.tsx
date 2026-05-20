import type { Metadata } from "next"
import { cookies } from "next/headers"
import { getT } from "@/i18n/getT"
import { isValidLocale, defaultLocale, type Locale } from "@/i18n/routing"
import { AbuseClient } from "./abuse-client"

export const metadata: Metadata = { title: "Account Ban — idcd Admin" }

async function getAdminLocale(): Promise<Locale> {
  const cookieStore = await cookies()
  const val = cookieStore.get("locale")?.value ?? ""
  return isValidLocale(val) ? val : defaultLocale
}

export default async function CertAdminAbusePage() {
  const locale = await getAdminLocale()
  const t = await getT("admin", locale)
  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">{t("cert.abuse.title")}</h1>
        <p className="mt-1 text-sm text-muted-foreground">{t("cert.abuse.subtitle")}</p>
      </div>
      <AbuseClient />
    </div>
  )
}
