import Link from "next/link"
import { getTranslations } from "next-intl/server"
import { ScrollArea } from "@/components/ui/scroll-area"

/**
 * 文档侧边栏结构。
 *
 * - `heading` 来自 `docs.sidebar.{group}.heading`
 * - `items[].label` 来自 `docs.sidebar.{group}.items.{key}`
 *
 * URL 是路由的事实，所以保留在代码里；展示文本走 i18n。
 */
const NAV_STRUCTURE = [
  {
    group: "intro",
    items: [
      { key: "home", href: "/docs" },
      { key: "gettingStarted", href: "/docs/getting-started" },
    ],
  },
  {
    group: "tools",
    items: [
      { key: "http",  href: "/docs/tools/http" },
      { key: "ping",  href: "/docs/tools/ping" },
      { key: "dns",   href: "/docs/tools/dns" },
      { key: "ssl",   href: "/docs/tools/ssl" },
      { key: "whois", href: "/docs/tools/whois" },
    ],
  },
  {
    group: "mcp",
    items: [
      { key: "intro",          href: "/docs/mcp" },
      { key: "quickstart",     href: "/docs/mcp/quickstart" },
      { key: "authentication", href: "/docs/mcp/authentication" },
      { key: "tools",          href: "/docs/mcp/tools" },
      { key: "claudeCode",     href: "/docs/mcp/examples/claude-code" },
      { key: "cursor",         href: "/docs/mcp/examples/cursor" },
      { key: "python",         href: "/docs/mcp/examples/python" },
    ],
  },
] as const

export default async function DocsLayout({ children }: { children: React.ReactNode }) {
  const t = await getTranslations("docs.sidebar")
  return (
    <div className="flex min-h-[calc(100dvh-4rem)]">
      <aside className="hidden w-56 shrink-0 border-r lg:block">
        <ScrollArea className="h-full py-6 pr-4 pl-6">
          {NAV_STRUCTURE.map(section => (
            <div key={section.group} className="mb-6">
              <p className="mb-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                {t(`${section.group}.heading`)}
              </p>
              <div className="space-y-0.5">
                {section.items.map(item => (
                  <Link
                    key={item.href}
                    href={item.href}
                    className="block rounded px-2 py-1.5 text-sm text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
                  >
                    {t(`${section.group}.items.${item.key}`)}
                  </Link>
                ))}
              </div>
            </div>
          ))}
        </ScrollArea>
      </aside>

      <main className="flex-1 overflow-auto">
        <div className="mx-auto max-w-3xl px-6 py-10">
          {children}
        </div>
      </main>
    </div>
  )
}
