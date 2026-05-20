import type { Metadata } from "next"
import { getT } from "@/i18n/getT"
import { StatusPagesClient } from "./status-pages-client"

export async function generateMetadata(): Promise<Metadata> {
  const t = await getT("status")
  return {
    title: `${t("statusPages.title")} - idcd`,
    description: t("statusPages.description"),
  }
}

export default async function StatusPagesPage() {
  const t = await getT("status")
  return (
    <>
      <div>
        <h1 className="text-2xl font-bold tracking-tight">{t("statusPages.title")}</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          {t("statusPages.description")}
        </p>
      </div>
      <StatusPagesClient />
    </>
  )
}
