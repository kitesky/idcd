"use client"

import Link from "next/link"
import { usePathname } from "next/navigation"
import { useState } from "react"
import {
  Card,
  CardContent,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  cn
} from "@/components/ui"
import { Menu, X } from "lucide-react"
import { ALL_TOOLS } from "@/app/tools/tools-config"

const STATIC_TOOLS = [
  { slug: 'diagnose', name: '一键诊断' },
  { slug: 'http', name: 'HTTP/HTTPS 拨测' },
  { slug: 'ping', name: '多地 Ping' },
  { slug: 'tcping', name: 'TCP 端口测试' },
  { slug: 'dns', name: 'DNS 解析' },
  { slug: 'traceroute', name: '路由追踪' },
  { slug: 'json-formatter', name: 'JSON 格式化' },
  { slug: 'base64', name: 'Base64 编解码' },
  { slug: 'timestamp', name: '时间戳转换' },
  { slug: 'hash', name: '哈希计算' },
  { slug: 'jwt-decoder', name: 'JWT 解码' },
  { slug: 'regex-tester', name: '正则表达式测试' },
  { slug: 'cron-parser', name: 'Cron 表达式解析' },
  { slug: 'qrcode', name: '二维码生成' },
  { slug: 'cidr-calculator', name: 'IP 段 / CIDR 计算' },
  { slug: 'ipv6-converter', name: 'IPv6 检测 / 转换' },
]

const NEW_TOOL_GROUPS = [
  {
    label: '拨测工具（新）',
    slugs: ['ssl','whois','icp','ip','tcp','mtr','smtp','rdns','asn','mx','spf','dmarc','ntp','dkim','bgp'],
  },
  {
    label: '文本工具',
    slugs: ['word-counter','line-sort','duplicate-remover','text-case','html-encode','escape-html','text-stats','diff','markdown'],
  },
  {
    label: '转换工具',
    slugs: ['url-encode','unicode','jwt-decode','number-convert','json-to-yaml','yaml-formatter','xml-formatter','url-parser','user-agent','number-format'],
  },
  {
    label: '生成工具',
    slugs: ['password-gen','uuid-gen','lorem','chmod-calc','sort-json','color-picker','image-base64'],
  },
  {
    label: '查询工具',
    slugs: ['regex','cron-viz','cidr-calc','ipv6-check','http-status','mime-type','timezone','date-calc','csv-formatter'],
  },
]

const ALL_SELECT_TOOLS = [
  ...STATIC_TOOLS,
  ...ALL_TOOLS.map(t => ({ slug: t.slug, name: t.name })),
]

interface ToolsLayoutProps {
  children: React.ReactNode
}

export default function ToolsLayout({ children }: ToolsLayoutProps) {
  const pathname = usePathname()
  const [sidebarOpen, setSidebarOpen] = useState(false)
  const [showNew, setShowNew] = useState(false)

  const currentSlug = pathname.split('/tools/')[1]?.split('/')[0] ?? ''

  return (
    <div className="container mx-auto px-4 py-8">
      <div className="flex flex-col lg:flex-row gap-6">
        {/* Mobile: Hamburger + Tool selector */}
        <div className="lg:hidden">
          <div className="flex items-center justify-between mb-4">
            <h1 className="text-2xl font-bold">开发工具</h1>
            <button
              onClick={() => setSidebarOpen(!sidebarOpen)}
              className="p-2 rounded-md border"
            >
              {sidebarOpen ? <X className="h-5 w-5" /> : <Menu className="h-5 w-5" />}
            </button>
          </div>

          {!sidebarOpen && (
            <Select value={currentSlug} onValueChange={(value) => {
              window.location.href = `/tools/${value}`
            }}>
              <SelectTrigger className="w-full">
                <SelectValue placeholder="选择工具" />
              </SelectTrigger>
              <SelectContent>
                {ALL_SELECT_TOOLS.map((tool) => (
                  <SelectItem key={tool.slug} value={tool.slug}>
                    {tool.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
        </div>

        {/* Desktop: Fixed sidebar OR Mobile: Collapsible sidebar */}
        <div className={cn(
          "lg:w-64 lg:flex-shrink-0",
          "lg:block",
          sidebarOpen ? "block" : "hidden lg:block"
        )}>
          <Card>
            <CardContent className="p-4">
              <h2 className="font-semibold text-lg mb-3 hidden lg:block">开发工具</h2>

              {/* 原有工具 */}
              <nav className="space-y-0.5">
                {STATIC_TOOLS.map((tool) => {
                  const isActive = currentSlug === tool.slug
                  return (
                    <Link
                      key={tool.slug}
                      href={`/tools/${tool.slug}` as any}
                      className={cn(
                        "block px-2 py-1.5 rounded text-sm transition-colors",
                        isActive
                          ? "bg-primary text-primary-foreground"
                          : "text-muted-foreground hover:text-foreground hover:bg-muted"
                      )}
                      onClick={() => setSidebarOpen(false)}
                    >
                      {tool.name}
                    </Link>
                  )
                })}
              </nav>

              {/* 折叠新工具 */}
              <button
                onClick={() => setShowNew(prev => !prev)}
                className="mt-3 w-full text-xs text-muted-foreground hover:text-foreground transition-colors px-2 py-1 text-left border rounded"
              >
                {showNew ? '▾ 收起 50 个新工具' : '▸ 展开 50 个新工具'}
              </button>

              {showNew && NEW_TOOL_GROUPS.map(group => (
                <div key={group.label} className="mt-3">
                  <p className="text-xs text-muted-foreground uppercase tracking-wide mb-1 px-1">{group.label}</p>
                  <nav className="space-y-0.5">
                    {group.slugs.map(slug => {
                      const tool = ALL_TOOLS.find(t => t.slug === slug)
                      if (!tool) return null
                      const isActive = currentSlug === slug
                      return (
                        <Link
                          key={slug}
                          href={`/tools/${slug}` as any}
                          className={cn(
                            "block px-2 py-1.5 rounded text-sm transition-colors",
                            isActive
                              ? "bg-primary text-primary-foreground"
                              : "text-muted-foreground hover:text-foreground hover:bg-muted"
                          )}
                          onClick={() => setSidebarOpen(false)}
                        >
                          {tool.name}
                        </Link>
                      )
                    })}
                  </nav>
                </div>
              ))}
            </CardContent>
          </Card>
        </div>

        {/* Main content area */}
        <div className="flex-1 min-w-0">
          {children}
        </div>
      </div>
    </div>
  )
}
