import type { Metadata } from "next"
import { getT } from "@/i18n/getT"
import { NewMonitorClient } from "./new-monitor-client"

export async function generateMetadata(): Promise<Metadata> {
  const t = await getT("monitors")
  return {
    title: `${t("new.title")} - idcd`,
    description: t("new.description"),
  }
}

export default async function NewMonitorPage() {
  const t = await getT("monitors")

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
