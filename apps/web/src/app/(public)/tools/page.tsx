import type { Metadata } from "next"
import Link from "next/link"

export const metadata: Metadata = {
  title: "网络诊断工具箱 | idcd",
  description: "50+ 在线工具，网络诊断 · 格式转换 · 文本处理 · 生成和查询，从左侧分类浏览。",
}

const SECTIONS = [
  {
    id: "probe",
    label: "拨测检测",
    desc: "从全球节点检测延迟、证书和 DNS",
    tools: [
      { slug: "diagnose",   name: "一键诊断" },
      { slug: "http",       name: "HTTP 拨测" },
      { slug: "ping",       name: "多地 Ping" },
      { slug: "ssl",        name: "SSL 证书" },
      { slug: "dns",        name: "DNS 解析" },
      { slug: "whois",      name: "WHOIS 查询" },
      { slug: "traceroute", name: "路由追踪" },
      { slug: "tcping",     name: "TCPing" },
    ],
  },
  {
    id: "convert",
    label: "格式转换",
    desc: "编解码、格式化与互转",
    tools: [
      { slug: "json-formatter", name: "JSON 格式化" },
      { slug: "base64",         name: "Base64" },
      { slug: "timestamp",      name: "时间戳" },
      { slug: "url-encode",     name: "URL 编解码" },
      { slug: "json-to-yaml",   name: "JSON ↔ YAML" },
      { slug: "jwt-decoder",    name: "JWT 解码" },
    ],
  },
  {
    id: "text",
    label: "文本处理",
    desc: "文本对比、统计与编辑",
    tools: [
      { slug: "diff",              name: "文本对比" },
      { slug: "markdown",          name: "Markdown" },
      { slug: "word-counter",      name: "字数统计" },
      { slug: "text-case",         name: "大小写转换" },
      { slug: "duplicate-remover", name: "去重工具" },
      { slug: "text-stats",        name: "文本统计" },
    ],
  },
  {
    id: "generate",
    label: "生成工具",
    desc: "密码、UUID、二维码与哈希",
    tools: [
      { slug: "password-gen", name: "密码生成" },
      { slug: "uuid-gen",     name: "UUID 生成" },
      { slug: "qrcode",       name: "二维码" },
      { slug: "hash",         name: "哈希计算" },
      { slug: "color-picker", name: "颜色转换" },
      { slug: "lorem",        name: "Lorem Ipsum" },
    ],
  },
  {
    id: "lookup",
    label: "查询工具",
    desc: "正则、状态码、子网与时区",
    tools: [
      { slug: "regex-tester",    name: "正则测试" },
      { slug: "http-status",     name: "HTTP 状态码" },
      { slug: "cidr-calculator", name: "CIDR 计算" },
      { slug: "timezone",        name: "时区转换" },
      { slug: "mime-type",       name: "MIME 类型" },
      { slug: "cron-parser",     name: "Cron 解析" },
    ],
  },
]

export default function ToolsPage() {
  return (
    <div className="mx-auto max-w-2xl px-6 py-8">

      {/* Header */}
      <div className="mb-8">
        <h1 className="text-lg font-semibold tracking-tight">工具箱</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          从左侧选择分类和工具，或点击下方快捷入口
        </p>
      </div>

      {/* Quick-access grid by section */}
      <div className="space-y-7">
        {SECTIONS.map(sec => (
          <section key={sec.id}>
            <div className="mb-2.5 flex items-baseline gap-2">
              <h2 className="text-[13px] font-semibold">{sec.label}</h2>
              <span className="text-xs text-muted-foreground">{sec.desc}</span>
            </div>
            <div className="grid grid-cols-2 gap-1.5 sm:grid-cols-3">
              {sec.tools.map(tool => (
                <Link
                  key={tool.slug}
                  href={`/tools/${tool.slug}` as any}
                  className="rounded-md border bg-card px-3 py-2.5 text-[13px] font-medium text-foreground transition-colors hover:border-primary hover:bg-primary/5 hover:text-primary"
                >
                  {tool.name}
                </Link>
              ))}
            </div>
          </section>
        ))}
      </div>
    </div>
  )
}
