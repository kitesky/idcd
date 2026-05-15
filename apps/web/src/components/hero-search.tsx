"use client"

import { useState } from "react"
import { useRouter } from "next/navigation"
import { ChevronDown, Search, Settings2, ArrowRight } from "lucide-react"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

// ── 大类 tab 配置 ────────────────────────────────────────────────────────────

interface CategoryTab {
  key: string
  label: string
  title: string
  subtitle: string
  desc: string
  tools: ToolTab[]
  inputPlaceholder: string
  showNodeSelector: boolean
}

interface ToolTab {
  key: string
  label: string
  path: string
}

const CATEGORIES: CategoryTab[] = [
  {
    key: "diagnose",
    label: "一键诊断",
    title: "一键综合诊断",
    subtitle: "HTTP • Ping • DNS • SSL • 路由追踪",
    desc: "一次输入，同时发起多项检测，快速定位域名/IP 的网络连通性和配置问题。",
    tools: [],
    inputPlaceholder: "请输入域名或 IP，如 github.com",
    showNodeSelector: false,
  },
  {
    key: "probe",
    label: "网络拨测",
    title: "网络拨测工具",
    subtitle: "多节点实时拨测 • 网络连通性诊断",
    desc: "全球 100+ 网络拨测节点，模拟用户访问域名/IP，帮助发现网络、站点可用性问题。",
    tools: [
      { key: "http",       label: "HTTP",       path: "/tools/http" },
      { key: "ping",       label: "Ping",       path: "/tools/ping" },
      { key: "dns",        label: "DNS",        path: "/tools/dns" },
      { key: "tcp",        label: "TCP",        path: "/tools/tcp" },
      { key: "mtr",        label: "MTR",        path: "/tools/mtr" },
      { key: "traceroute", label: "Traceroute", path: "/tools/traceroute" },
    ],
    inputPlaceholder: "请输入域名或 IP",
    showNodeSelector: true,
  },
  {
    key: "domain",
    label: "域名查询",
    title: "域名信息查询",
    subtitle: "WHOIS • ICP • SSL • DNS 全方位查询",
    desc: "查询域名注册信息、ICP 备案、SSL 证书状态及 DNS 解析记录。",
    tools: [
      { key: "whois", label: "WHOIS", path: "/tools/whois" },
      { key: "icp",   label: "ICP 备案", path: "/tools/icp" },
      { key: "ssl",   label: "SSL 证书", path: "/tools/ssl" },
      { key: "dns",   label: "DNS 解析", path: "/tools/dns" },
    ],
    inputPlaceholder: "请输入域名，如 example.com",
    showNodeSelector: false,
  },
]

// ── 节点区域列表 ──────────────────────────────────────────────────────────────

const NODE_REGIONS = [
  { value: "global", label: "全球节点" },
  { value: "cn-east", label: "中国华东" },
  { value: "cn-south", label: "中国华南" },
  { value: "cn-north", label: "中国华北" },
  { value: "hk", label: "香港" },
  { value: "sg", label: "新加坡" },
  { value: "us-west", label: "美国西部" },
  { value: "eu", label: "欧洲" },
]

// ── NodeSelector ─────────────────────────────────────────────────────────────

function NodeSelector({
  value,
  onChange,
}: {
  value: string
  onChange: (v: string) => void
}) {
  const [open, setOpen] = useState(false)
  const current = NODE_REGIONS.find(r => r.value === value) ?? NODE_REGIONS[0]!

  return (
    <div className="relative">
      <button
        type="button"
        onClick={() => setOpen(v => !v)}
        className="flex h-11 w-full items-center justify-between gap-2 px-4 text-sm text-foreground/80 hover:text-foreground transition-colors"
      >
        <span className="truncate">{current.label}</span>
        <ChevronDown className={cn("h-4 w-4 flex-shrink-0 text-muted-foreground transition-transform duration-150", open && "rotate-180")} />
      </button>

      {open && (
        <>
          <div className="fixed inset-0 z-40" onClick={() => setOpen(false)} />
          <div className="absolute top-full left-0 mt-1 z-50 min-w-[160px] bg-popover border rounded-lg shadow-lg py-1">
            {NODE_REGIONS.map(r => (
              <button
                key={r.value}
                type="button"
                onClick={() => { onChange(r.value); setOpen(false) }}
                className={cn(
                  "block w-full px-4 py-2 text-left text-sm transition-colors",
                  r.value === value
                    ? "text-primary bg-muted/60"
                    : "text-foreground hover:bg-muted"
                )}
              >
                {r.label}
              </button>
            ))}
          </div>
        </>
      )}
    </div>
  )
}

// ── HeroSearch ────────────────────────────────────────────────────────────────

export function HeroSearch() {
  const router = useRouter()
  const [activeCat, setActiveCat] = useState("probe")
  const [activeTool, setActiveTool] = useState("http")
  const [nodeRegion, setNodeRegion] = useState("global")
  const [query, setQuery] = useState("")

  const cat = CATEGORIES.find(c => c.key === activeCat) ?? CATEGORIES[1]!
  const tool = cat.tools.find(t => t.key === activeTool) ?? cat.tools[0]

  const handleCatChange = (key: string) => {
    setActiveCat(key)
    const c = CATEGORIES.find(c => c.key === key)
    if (c?.tools.length) setActiveTool(c.tools[0]!.key)
  }

  const handleGo = () => {
    if (!query.trim()) return
    const path = activeCat === "diagnose"
      ? `/tools/diagnose?q=${encodeURIComponent(query.trim())}`
      : tool
        ? `${tool.path}?q=${encodeURIComponent(query.trim())}`
        : `/tools/diagnose?q=${encodeURIComponent(query.trim())}`
    router.push(path as any)
  }

  return (
    <section className="relative bg-muted/30 border-b">
      {/* 顶部大类 tabs */}
      <div className="border-b">
        <div className="mx-auto max-w-screen-xl px-6">
          <div className="flex">
            {CATEGORIES.map(c => (
              <button
                key={c.key}
                onClick={() => handleCatChange(c.key)}
                className={cn(
                  "relative px-6 py-4 text-sm font-medium transition-colors",
                  activeCat === c.key
                    ? "text-primary after:absolute after:bottom-0 after:left-0 after:right-0 after:h-0.5 after:bg-primary"
                    : "text-muted-foreground hover:text-foreground"
                )}
              >
                {c.label}
              </button>
            ))}
          </div>
        </div>
      </div>

      {/* 主体内容 */}
      <div className="mx-auto max-w-screen-xl px-6 py-10">
        {/* 标题区 */}
        <div className="text-center mb-8">
          <h2 className="text-2xl font-bold text-foreground">{cat.title}</h2>
          <p className="mt-2 text-sm text-primary font-medium">{cat.subtitle}</p>
          <p className="mt-2 text-sm text-muted-foreground max-w-xl mx-auto">{cat.desc}</p>
        </div>

        {/* 工具子 tabs */}
        {cat.tools.length > 0 && (
          <div className="flex items-center gap-0 mb-5">
            {cat.tools.map(t => (
              <button
                key={t.key}
                onClick={() => setActiveTool(t.key)}
                className={cn(
                  "px-5 py-2 text-sm font-medium transition-colors border-b-2",
                  activeTool === t.key
                    ? "text-primary border-primary"
                    : "text-muted-foreground border-transparent hover:text-foreground"
                )}
              >
                {t.label}
              </button>
            ))}
          </div>
        )}

        {/* 输入行 */}
        <div className="flex gap-0 rounded-md border border-border overflow-hidden bg-background shadow-sm">
          {/* 节点选择器 */}
          {cat.showNodeSelector && (
            <>
              <div className="w-52 flex-shrink-0 border-r">
                <NodeSelector value={nodeRegion} onChange={setNodeRegion} />
              </div>
            </>
          )}

          {/* 域名 / IP 输入框 */}
          <div className="flex flex-1 items-center gap-2 px-4">
            <Search className="h-4 w-4 text-muted-foreground flex-shrink-0" />
            <input
              type="text"
              value={query}
              onChange={e => setQuery(e.target.value)}
              onKeyDown={e => e.key === "Enter" && handleGo()}
              placeholder={cat.inputPlaceholder}
              className="flex-1 h-11 bg-transparent text-sm outline-none placeholder:text-muted-foreground/60"
            />
          </div>

          {/* CTA */}
          <Button
            onClick={handleGo}
            disabled={!query.trim()}
            className="h-11 rounded-none px-6 text-sm font-medium"
          >
            立即检测
            <ArrowRight className="h-4 w-4 ml-1" />
          </Button>
        </div>

        {/* 底部辅助行 */}
        <div className="flex items-center justify-between mt-3 px-0.5">
          <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
            <Settings2 className="h-3.5 w-3.5" />
            <span>高级配置</span>
            <span className="mx-2 opacity-30">|</span>
            <span>
              <a href="/app/monitors/new" className="text-primary hover:underline">
                创建定时监测任务
              </a>
              ，持续监测不同地区用户到站点的访问连通性
            </span>
          </div>
          <a href="/tools/diagnose" className="text-xs text-primary hover:underline">
            对比检测
          </a>
        </div>
      </div>
    </section>
  )
}
