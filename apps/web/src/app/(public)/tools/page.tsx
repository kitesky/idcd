"use client"

import { useState, useMemo } from "react"
import Link from "next/link"
import { Search } from "lucide-react"
import { useTranslations } from "next-intl"
import { Card, CardContent, CardHeader, CardTitle, Badge, Input, Button } from "@/components/ui"
import { ALL_TOOLS } from "./tools-config"

const CATEGORIES: ReadonlyArray<'probe' | 'utility'> = ['probe', 'utility'] as const

function groupByCategory(tools: typeof ALL_TOOLS) {
  const map: Record<string, typeof ALL_TOOLS> = {}
  for (const tool of tools) {
    const existing = map[tool.category]
    if (existing) {
      existing.push(tool)
    } else {
      map[tool.category] = [tool]
    }
  }
  return map
}

export default function ToolsPage() {
  const t = useTranslations('tools')
  const [query, setQuery] = useState("")

  const filteredTools = useMemo(() => {
    const q = query.trim().toLowerCase()
    if (!q) return null
    return ALL_TOOLS.filter((tool) => {
      const title = String(t(`${tool.slug}.title`) ?? '').toLowerCase()
      const description = String(t(`${tool.slug}.description`) ?? '').toLowerCase()
      return title.includes(q) || description.includes(q) || tool.slug.includes(q)
    })
  }, [query, t])

  const grouped = useMemo(() => groupByCategory(ALL_TOOLS), [])

  function scrollToCategory(cat: string) {
    const el = document.getElementById(`cat-${cat}`)
    if (el) el.scrollIntoView({ behavior: "smooth", block: "start" })
  }

  return (
    <div className="mx-auto max-w-screen-xl px-4 py-12 sm:px-6 lg:px-8">
      <div className="text-center">
        <div className="flex items-center justify-center gap-3">
          <h1 className="text-4xl font-bold tracking-tight">{t('_page.title')}</h1>
          <Badge variant="outline" className="text-sm">
            {t('_page.toolCount', { count: ALL_TOOLS.length })}
          </Badge>
        </div>
        <p className="mt-4 text-lg text-muted-foreground">
          {t('_page.subtitle')}
        </p>
      </div>

      <div className="relative mx-auto mt-8 max-w-lg">
        <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          placeholder={t('_page.searchPlaceholder')}
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          className="pl-9"
        />
      </div>

      {!filteredTools && (
        <div className="mt-8 flex flex-wrap gap-2 justify-center">
          {CATEGORIES.map((cat) => (
            <button
              key={cat}
              onClick={() => scrollToCategory(cat)}
              className="inline-flex items-center"
            >
              <Badge
                variant="outline"
                className="cursor-pointer px-3 py-1 text-sm hover:border-primary hover:text-primary transition-colors"
              >
                {t(`_page.categories.${cat}.label`)}
                <span className="ml-1.5 text-muted-foreground">
                  {grouped[cat]?.length ?? 0}
                </span>
              </Badge>
            </button>
          ))}
        </div>
      )}

      {filteredTools !== null ? (
        <div className="mt-10">
          {filteredTools.length === 0 ? (
            <p className="text-center text-muted-foreground">
              {t('_page.noResult', { query })}
            </p>
          ) : (
            <>
              <p className="mb-4 text-sm text-muted-foreground">
                {t('_page.resultCount', { count: filteredTools.length })}
              </p>
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
                {filteredTools.map((tool) => (
                  <Card
                    key={tool.slug}
                    className="hover:shadow-md transition-shadow"
                  >
                    <CardHeader className="pb-2">
                      <div className="flex items-start justify-between gap-2">
                        <CardTitle className="text-base">{t(`${tool.slug}.title`)}</CardTitle>
                        <Badge variant="outline" className="shrink-0 text-xs">
                          {t(`_page.categories.${tool.category}.label`)}
                        </Badge>
                      </div>
                    </CardHeader>
                    <CardContent>
                      <p className="mb-4 text-sm text-muted-foreground">
                        {t(`${tool.slug}.description`)}
                      </p>
                      <Button asChild variant="outline" size="sm" className="w-full">
                        <Link href={`/tools/${tool.slug}`}>{t('_page.useNow')}</Link>
                      </Button>
                    </CardContent>
                  </Card>
                ))}
              </div>
            </>
          )}
        </div>
      ) : (
        <div className="mt-12 space-y-16">
          {CATEGORIES.filter((cat) => grouped[cat]?.length).map((cat) => (
            <section key={cat} id={`cat-${cat}`}>
              <div className="mb-6">
                <h2 className="text-2xl font-bold tracking-tight">
                  {t(`_page.categories.${cat}.label`)}
                </h2>
                <p className="mt-1 text-muted-foreground">
                  {t(`_page.categories.${cat}.description`)}
                </p>
              </div>
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
                {(grouped[cat] ?? []).map((tool) => (
                  <Card
                    key={tool.slug}
                    className="hover:shadow-md transition-shadow"
                  >
                    <CardHeader className="pb-2">
                      <CardTitle className="text-base">{t(`${tool.slug}.title`)}</CardTitle>
                    </CardHeader>
                    <CardContent>
                      <p className="mb-4 text-sm text-muted-foreground">
                        {t(`${tool.slug}.description`)}
                      </p>
                      <Button asChild variant="outline" size="sm" className="w-full">
                        <Link href={`/tools/${tool.slug}`}>{t('_page.useNow')}</Link>
                      </Button>
                    </CardContent>
                  </Card>
                ))}
              </div>
            </section>
          ))}
        </div>
      )}
    </div>
  )
}
