import type { Metadata } from "next"
import { getTranslations } from "next-intl/server"
import { getLocale } from "@/i18n/locale"
import { NewMonitorClient } from "./new-monitor-client"

export async function generateMetadata(): Promise<Metadata> {
  const locale = await getLocale()
  const t = await getTranslations({ locale, namespace: "monitors" })
  return {
    title: `${t("new.title")} - idcd`,
    description: t("new.description"),
  }
}

export default async function NewMonitorPage() {
  const locale = await getLocale()
  const t = await getTranslations({ locale, namespace: "monitors" })

  return (
    <>
      <div>
        <h1 className="text-2xl font-bold tracking-tight">{t("new.title")}</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          {t("new.description")}
        </p>
      </div>
      <NewMonitorClient />
    </>
  )
}
