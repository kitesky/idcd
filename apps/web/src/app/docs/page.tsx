import type { Metadata } from "next"
import Link from "next/link"
import { Card, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"

export const metadata: Metadata = {
  title: "文档 | idcd",
  description: "idcd 网络诊断工具平台文档，包含工具使用说明、MCP Server 接入指南等。",
}

const SECTIONS = [
  {
    title: "快速开始",
    description: "5 分钟了解 idcd 平台功能和基本使用方式",
    href: "/docs/getting-started",
  },
  {
    title: "工具文档",
    description: "HTTP 拨测、Ping、DNS、SSL 等全部工具的参数和使用说明",
    href: "/docs/tools/http",
  },
  {
    title: "MCP Server",
    description: "让 Claude、Cursor 等 AI 工具直接调用全球网络拨测能力",
    href: "/docs/mcp",
  },
]

export default function DocsPage() {
  return (
    <div>
      <div className="mb-10">
        <h1 className="text-3xl font-bold tracking-tight">idcd 文档</h1>
        <p className="mt-3 text-muted-foreground">欢迎使用 idcd 网络诊断工具平台。</p>
      </div>

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {SECTIONS.map(s => (
          <Link key={s.href} href={s.href as any}>
            <Card className="h-full transition-colors hover:border-primary hover:bg-primary/5">
              <CardHeader>
                <CardTitle className="text-base">{s.title}</CardTitle>
                <CardDescription>{s.description}</CardDescription>
              </CardHeader>
            </Card>
          </Link>
        ))}
      </div>

      <Separator className="my-10" />

      <div>
        <h2 className="mb-4 text-lg font-semibold">主要功能</h2>
        <ul className="space-y-2 text-sm text-muted-foreground">
          <li>• <strong className="text-foreground">HTTP/HTTPS 拨测</strong> — 检测网站可达性、响应时间、状态码</li>
          <li>• <strong className="text-foreground">Ping 测试</strong> — 测试延迟、丢包率</li>
          <li>• <strong className="text-foreground">TCPing</strong> — TCP 端口连通性和响应时间</li>
          <li>• <strong className="text-foreground">DNS 查询</strong> — 多地 DNS 解析结果对比</li>
          <li>• <strong className="text-foreground">路由追踪</strong> — 查看数据包路径，定位延迟节点</li>
          <li>• <strong className="text-foreground">MCP Server</strong> — AI 工具直接调用拨测能力</li>
        </ul>
      </div>
    </div>
  )
}

function Separator({ className }: { className?: string }) {
  return <hr className={className} />
}
