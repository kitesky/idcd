"use client"

import { useState, useMemo } from "react"
import Link from "next/link"
import { Search } from "lucide-react"
import { Card, CardContent, CardHeader, CardTitle, Badge, Input } from "@/components/ui"
import { ALL_TOOLS } from "./tools-config"

const CATEGORY_META: Record<string, { label: string; description: string }> = {
  probe: {
    label: "拨测检测",
    description: "从全球节点检测延迟、证书、DNS 和协议可用性",
  },
  utility: {
    label: "实用工具",
    description: "格式转换、编解码、文本处理、生成与查询",
  },
}

const CATEGORIES = Object.keys(CATEGORY_META)

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
  const [query, setQuery] = useState("")

  const filteredTools = useMemo(() => {
    const q = query.trim().toLowerCase()
    if (!q) return null
    return ALL_TOOLS.filter(
      (t) =>
        t.name.toLowerCase().includes(q) ||
        t.description.toLowerCase().includes(q)
    )
  }, [query])

  const grouped = useMemo(() => groupByCategory(ALL_TOOLS), [])

  function scrollToCategory(cat: string) {
    const el = document.getElementById(`cat-${cat}`)
    if (el) el.scrollIntoView({ behavior: "smooth", block: "start" })
  }

  return (
    <div className="mx-auto max-w-screen-xl px-4 py-12 sm:px-6 lg:px-8">
      <div className="text-center">
        <div className="flex items-center justify-center gap-3">
          <h1 className="text-4xl font-bold tracking-tight">专业网络工具站</h1>
          <Badge variant="outline" className="text-sm">
            {ALL_TOOLS.length} 个工具
          </Badge>
        </div>
        <p className="mt-4 text-lg text-muted-foreground">
          拨测检测、格式转换、文本处理、生成与查询，一站搞定
        </p>
      </div>

      <div className="relative mx-auto mt-8 max-w-lg">
        <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          placeholder="搜索工具名称或描述…"
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
                {CATEGORY_META[cat]?.label ?? cat}
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
              未找到 &ldquo;{query}&rdquo;，试试其他关键词
            </p>
          ) : (
            <>
              <p className="mb-4 text-sm text-muted-foreground">
                找到 {filteredTools.length} 个结果
              </p>
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
                {filteredTools.map((tool) => (
                  <Card
                    key={tool.slug}
                    className="hover:shadow-md transition-shadow"
                  >
                    <CardHeader className="pb-2">
                      <div className="flex items-start justify-between gap-2">
                        <CardTitle className="text-base">{tool.name}</CardTitle>
                        <Badge variant="outline" className="shrink-0 text-xs">
                          {CATEGORY_META[tool.category]?.label ?? tool.category}
                        </Badge>
                      </div>
                    </CardHeader>
                    <CardContent>
                      <p className="mb-4 text-sm text-muted-foreground">
                        {tool.description}
                      </p>
                      <Link
                        href={`/tools/${tool.slug}` as any}
                        className="inline-flex h-8 items-center justify-center rounded-md border border-input bg-background px-3 text-sm font-medium ring-offset-background transition-colors hover:bg-accent hover:text-accent-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 w-full"
                      >
                        立即使用
                      </Link>
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
                  {CATEGORY_META[cat]?.label ?? cat}
                </h2>
                <p className="mt-1 text-muted-foreground">
                  {CATEGORY_META[cat]?.description}
                </p>
              </div>
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
                {(grouped[cat] ?? []).map((tool) => (
                  <Card
                    key={tool.slug}
                    className="hover:shadow-md transition-shadow"
                  >
                    <CardHeader className="pb-2">
                      <CardTitle className="text-base">{tool.name}</CardTitle>
                    </CardHeader>
                    <CardContent>
                      <p className="mb-4 text-sm text-muted-foreground">
                        {tool.description}
                      </p>
                      <Link
                        href={`/tools/${tool.slug}` as any}
                        className="inline-flex h-8 items-center justify-center rounded-md border border-input bg-background px-3 text-sm font-medium ring-offset-background transition-colors hover:bg-accent hover:text-accent-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 w-full"
                      >
                        立即使用
                      </Link>
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
