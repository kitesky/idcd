import type { Metadata } from "next"
import { LeaderboardClient } from "./leaderboard-client"
import { NODE_COUNT, getCurrentMonthLabel } from "./leaderboard-data"
import { getT } from "@/i18n/getT"
import { getLocale } from "@/i18n/locale"
import { generateAlternates, localizedUrl } from "@/lib/seo"

export async function generateMetadata(): Promise<Metadata> {
  const locale = await getLocale()
  const t = await getT("leaderboard", locale)
  const title = t("meta.title")
  const description = t("meta.description")
  const keywords = t("meta.keywords")
    .split(",")
    .map((s) => s.trim())
    .filter(Boolean)
  return {
    title,
    description,
    keywords,
    alternates: generateAlternates("/leaderboard", locale),
    openGraph: {
      title,
      description: t("meta.ogDescription"),
      url: localizedUrl("/leaderboard", locale),
    },
  }
}

export default async function LeaderboardPage() {
  const locale = await getLocale()
  const t = await getT("leaderboard", locale)
  const monthLabel = getCurrentMonthLabel(locale, t("monthLabel"))

  return (
    <main className="min-h-screen bg-background">
      <div className="container mx-auto px-4 py-8 max-w-7xl">
        {/* Header */}
        <div className="mb-8">
          <h1 className="text-3xl font-bold tracking-tight">
            {t("title")}
          </h1>
          <p className="mt-2 text-muted-foreground">
            {t("subtitle", { nodeCount: NODE_COUNT })}
          </p>
          <p className="mt-1 text-sm text-muted-foreground">
            {t("dataCycleLine", { monthLabel })}
          </p>
        </div>

        {/* Tabs + Content */}
        <LeaderboardClient />
      </div>
    </main>
  )
}
