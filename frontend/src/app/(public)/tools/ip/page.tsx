import type { Metadata } from "next"
import { getT } from "@/i18n/getT"
import { getLocale } from "@/i18n/locale"
import { buildLocalizedMetadata } from "@/lib/seo"
import IpInfoClient from "./ip-info-client"

export async function generateMetadata(): Promise<Metadata> {
  const locale = await getLocale()
  const t = await getT("tools", locale)
  return buildLocalizedMetadata({
    path: "/tools/ip",
    locale,
    title: t("ip.meta.title"),
    description: t("ip.meta.description"),
  })
}

export default function IpPage() {
  return <IpInfoClient />
}
