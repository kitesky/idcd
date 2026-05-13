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

const tools = [
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

interface ToolsLayoutProps {
  children: React.ReactNode
}

export default function ToolsLayout({ children }: ToolsLayoutProps) {
  const pathname = usePathname()
  const [sidebarOpen, setSidebarOpen] = useState(false)

  const currentTool = tools.find(tool => pathname.includes(tool.slug))

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

          {/* Mobile tool selector */}
          {!sidebarOpen && (
            <Select value={currentTool?.slug} onValueChange={(value) => {
              window.location.href = `/tools/${value}`
            }}>
              <SelectTrigger className="w-full">
                <SelectValue placeholder="选择工具" />
              </SelectTrigger>
              <SelectContent>
                {tools.map((tool) => (
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
            <CardContent className="p-6">
              <h2 className="font-semibold text-lg mb-4 hidden lg:block">开发工具</h2>
              <nav className="space-y-2">
                {tools.map((tool) => {
                  const isActive = pathname.includes(tool.slug)
                  return (
                    <Link
                      key={tool.slug}
                      href={`/tools/${tool.slug}` as any}
                      className={cn(
                        "block px-3 py-2 rounded-md text-sm transition-colors",
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