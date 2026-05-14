import Link from "next/link"
import { ScrollArea } from "@/components/ui/scroll-area"
import { Separator } from "@/components/ui/separator"

const NAV = [
  { heading: "入门", items: [
    { href: "/docs", label: "文档首页" },
    { href: "/docs/getting-started", label: "快速开始" },
  ]},
  { heading: "工具文档", items: [
    { href: "/docs/tools/http",  label: "HTTP 拨测" },
    { href: "/docs/tools/ping",  label: "多地 Ping" },
    { href: "/docs/tools/dns",   label: "DNS 解析" },
    { href: "/docs/tools/ssl",   label: "SSL 证书" },
    { href: "/docs/tools/whois", label: "WHOIS 查询" },
  ]},
  { heading: "MCP Server", items: [
    { href: "/docs/mcp",                     label: "MCP 简介" },
    { href: "/docs/mcp/quickstart",          label: "快速接入" },
    { href: "/docs/mcp/authentication",      label: "认证配置" },
    { href: "/docs/mcp/tools",               label: "工具列表" },
    { href: "/docs/mcp/examples/claude-code",label: "Claude Code" },
    { href: "/docs/mcp/examples/cursor",     label: "Cursor IDE" },
    { href: "/docs/mcp/examples/python",     label: "Python SDK" },
  ]},
]

export default function DocsLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex min-h-[calc(100dvh-4rem)]">
      {/* Sidebar */}
      <aside className="hidden w-56 shrink-0 border-r lg:block">
        <ScrollArea className="h-full py-6 pr-4 pl-6">
          {NAV.map(section => (
            <div key={section.heading} className="mb-6">
              <p className="mb-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">{section.heading}</p>
              <div className="space-y-0.5">
                {section.items.map(item => (
                  <Link key={item.href} href={item.href as any}
                    className="block rounded px-2 py-1.5 text-sm text-muted-foreground transition-colors hover:bg-muted hover:text-foreground">
                    {item.label}
                  </Link>
                ))}
              </div>
            </div>
          ))}
        </ScrollArea>
      </aside>

      {/* Content */}
      <main className="flex-1 overflow-auto">
        <div className="mx-auto max-w-3xl px-6 py-10">
          {children}
        </div>
      </main>
    </div>
  )
}
