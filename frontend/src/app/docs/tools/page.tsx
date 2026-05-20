import type { Metadata } from "next"
import Link from "next/link"
import { getTranslations } from "next-intl/server"
import { Card, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"

const TOOL_KEYS = ["http", "ping", "dns", "ssl", "whois"] as const

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("docs.toolsIndex.meta")
  return { title: t("title") }
}

export default async function ToolsDocsPage() {
  const t = await getTranslations("docs.toolsIndex")
  return (
    <div>
      <h1 className="mb-2 text-3xl font-bold">{t("title")}</h1>
      <p className="mb-8 text-muted-foreground">{t("subtitle")}</p>
      <div className="grid gap-3 sm:grid-cols-2">
        {TOOL_KEYS.map(key => (
          <Link key={key} href={t(`items.${key}.href`)}>
            <Card className="h-full transition-colors hover:border-primary hover:bg-primary/5">
              <CardHeader>
                <CardTitle className="text-sm">{t(`items.${key}.title`)}</CardTitle>
                <CardDescription>{t(`items.${key}.description`)}</CardDescription>
              </CardHeader>
            </Card>
          </Link>
        ))}
      </div>
    </div>
  )
}
