import { Metadata } from "next"
import { getT } from "@/i18n/getT"
import { AlertsClient } from "./alerts-client"

export async function generateMetadata(): Promise<Metadata> {
  const t = await getT("alerts")
  return {
    title: `${t("title")} - idcd`,
    description: t("metaDescription"),
  }
}

export default async function AlertsPage() {
  const t = await getT("alerts")
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
