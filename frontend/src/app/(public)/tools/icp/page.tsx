import type { Metadata } from "next"
import { getT } from "@/i18n/getT"
import { getLocale } from "@/i18n/locale"
import { buildLocalizedMetadata } from "@/lib/seo"
import IcpInfoClient from "./icp-info-client"

export async function generateMetadata(): Promise<Metadata> {
  const locale = await getLocale()
  const t = await getT("tools", locale)
  return buildLocalizedMetadata({
    path: "/tools/icp",
    locale,
    title: t("icp.meta.title"),
    description: t("icp.meta.description"),
  })
}

export default function IcpPage() {
  return <IcpInfoClient />
}
