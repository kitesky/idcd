import type { Metadata } from "next"
import { getT } from "@/i18n/getT"
import { getLocale } from "@/i18n/locale"
import { buildLocalizedMetadata } from "@/lib/seo"
import SslInfoClient from "./ssl-info-client"

export async function generateMetadata(): Promise<Metadata> {
  const locale = await getLocale()
  const t = await getT("tools", locale)
  return buildLocalizedMetadata({
    path: "/tools/ssl",
    locale,
    title: t("ssl.meta.title"),
    description: t("ssl.meta.description"),
  })
}

export default function SslPage() {
  return <SslInfoClient />
}
