import type { Metadata } from "next"
import { getT } from "@/i18n/getT"
import { UsageClient } from "./usage-client"

export async function generateMetadata(): Promise<Metadata> {
  const t = await getT("billing.usage")
  return {
    title: `${t("title")} - idcd`,
    description: t("metaDescription"),
  }
}

export default async function UsagePage() {
  const t = await getT("billing.usage")
  return (
    <>
      <div>
        <h1 className="text-2xl font-bold tracking-tight">{t("title")}</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          {t("subtitle")}
        </p>
      </div>
      <UsageClient />
    </>
  )
}
