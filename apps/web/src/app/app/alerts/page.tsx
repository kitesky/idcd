import { Metadata } from "next"
import { getTranslations } from "next-intl/server"
import { AlertsClient } from "./alerts-client"

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("alerts")
  return {
    title: `${t("title")} - idcd`,
    description: t("metaDescription"),
  }
}

export default async function AlertsPage() {
  const t = await getTranslations("alerts")
  return (
    <>
      <div>
        <h1 className="text-2xl font-bold tracking-tight">{t("title")}</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          {t("description")}
        </p>
      </div>
      <AlertsClient />
    </>
  )
}
