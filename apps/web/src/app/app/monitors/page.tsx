import type { Metadata } from "next"
import { getTranslations } from "next-intl/server"
import { getLocale } from "@/i18n/locale"
import { MonitorsClient } from "./monitors-client"

export async function generateMetadata(): Promise<Metadata> {
  const locale = await getLocale()
  const t = await getTranslations({ locale, namespace: "monitors" })
  return {
    title: `${t("title")} - idcd`,
    description: t("description"),
  }
}

export default async function MonitorsPage() {
  const locale = await getLocale()
  const t = await getTranslations({ locale, namespace: "monitors" })

  return (
    <>
      <div>
        <h1 className="text-2xl font-bold tracking-tight">{t("title")}</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          {t("description")}
        </p>
      </div>
      <MonitorsClient />
    </>
  )
}
