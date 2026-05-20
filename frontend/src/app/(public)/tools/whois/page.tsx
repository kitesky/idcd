import type { Metadata } from "next"
import { getT } from "@/i18n/getT"
import { getLocale } from "@/i18n/locale"
import { buildLocalizedMetadata } from "@/lib/seo"
import WhoisInfoClient from "./whois-info-client"

export async function generateMetadata(): Promise<Metadata> {
  const locale = await getLocale()
  const t = await getT("tools", locale)
  return buildLocalizedMetadata({
    path: "/tools/whois",
    locale,
    title: t("whois.meta.title"),
    description: t("whois.meta.description"),
  })
}

export default function WhoisPage() {
  return <WhoisInfoClient />
}
