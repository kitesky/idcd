import type { Metadata } from "next"
import Link from "next/link"
import { Card, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"

export const metadata: Metadata = { title: "工具文档 | idcd" }

const TOOLS = [
  { href: "/docs/tools/http",  title: "HTTP/HTTPS 拨测", desc: "检测网站可达性、响应时间、状态码" },
  { href: "/docs/tools/ping",  title: "多地 Ping",       desc: "测试延迟、丢包率" },
  { href: "/docs/tools/dns",   title: "DNS 解析",        desc: "多地 DNS 解析结果对比" },
  { href: "/docs/tools/ssl",   title: "SSL 证书",        desc: "检测证书有效期和配置" },
  { href: "/docs/tools/whois", title: "WHOIS 查询",      desc: "查询域名注册信息" },
]

export default function ToolsDocsPage() {
  return (
    <div>
      <h1 className="mb-2 text-3xl font-bold">工具文档</h1>
      <p className="mb-8 text-muted-foreground">全部工具的参数说明和使用示例。</p>
      <div className="grid gap-3 sm:grid-cols-2">
        {TOOLS.map(t => (
          <Link key={t.href} href={t.href as any}>
            <Card className="h-full transition-colors hover:border-primary hover:bg-primary/5">
              <CardHeader><CardTitle className="text-sm">{t.title}</CardTitle><CardDescription>{t.desc}</CardDescription></CardHeader>
            </Card>
          </Link>
        ))}
      </div>
    </div>
  )
}
