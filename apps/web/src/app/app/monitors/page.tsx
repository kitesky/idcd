import type { Metadata } from "next"
import { getT } from "@/i18n/getT"
import { MonitorsClient } from "./monitors-client"

export async function generateMetadata(): Promise<Metadata> {
  const t = await getT("monitors")
  return {
    title: `${t("title")} - idcd`,
    description: t("description"),
  }
}

export default async function MonitorsPage() {
  const t = await getT("monitors")

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
