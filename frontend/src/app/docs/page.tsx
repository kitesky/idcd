import type { Metadata } from "next"
import Link from "next/link"
import { getTranslations } from "next-intl/server"
import { Card, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"
import { Separator } from "@/components/ui/separator"

const SECTION_KEYS = ["gettingStarted", "tools", "mcp"] as const
const FEATURE_KEYS = ["http", "ping", "tcping", "dns", "traceroute", "mcp"] as const

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("docs.meta")
  return {
    title: t("title"),
    description: t("description"),
  }
}

export default async function DocsPage() {
  const t = await getTranslations("docs.home")
  return (
    <div>
      <div className="mb-10">
        <h1 className="text-3xl font-bold tracking-tight">{t("title")}</h1>
        <p className="mt-3 text-muted-foreground">{t("subtitle")}</p>
      </div>

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {SECTION_KEYS.map(key => (
          <Link key={key} href={t(`sections.${key}.href`)}>
            <Card className="h-full transition-colors hover:border-primary hover:bg-primary/5">
              <CardHeader>
                <CardTitle className="text-base">{t(`sections.${key}.title`)}</CardTitle>
                <CardDescription>{t(`sections.${key}.description`)}</CardDescription>
              </CardHeader>
            </Card>
          </Link>
        ))}
      </div>

      <Separator className="my-10" />

      <div>
        <h2 className="mb-4 text-lg font-semibold">{t("featuresTitle")}</h2>
        <ul className="space-y-2 text-sm text-muted-foreground">
          {FEATURE_KEYS.map(key => (
            <li key={key}>
              {"• "}
              <strong className="text-foreground">{t(`features.${key}.name`)}</strong>
              {" — "}
              {t(`features.${key}.description`)}
            </li>
          ))}
        </ul>
      </div>
    </div>
  )
}
