"use client"

import { useState, useMemo, useEffect } from "react"
import { usePathname } from "next/navigation"
import Link from "next/link"
import { Search, Menu } from "lucide-react"
import {
  ScrollArea, Input, Badge, Button, Separator,
  Sheet, SheetContent, SheetTitle, cn,
} from "@/components/ui"
import { ALL_TOOLS } from "./tools-config"

// ─── Types ────────────────────────────────────────────────────────────────────

type ToolSubcategory = "probe" | "text" | "convert" | "generate" | "lookup"
type CategoryId = "all" | ToolSubcategory

interface SidebarTool {
  slug: string
  name: string
  description: string
  subcategory: ToolSubcategory
}

// ─── Categories ───────────────────────────────────────────────────────────────

const CATEGORIES: { id: CategoryId; label: string }[] = [
  { id: "all",      label: "全部工具" },
  { id: "probe",    label: "拨测检测" },
  { id: "convert",  label: "格式转换" },
  { id: "text",     label: "文本处理" },
  { id: "generate", label: "生成工具" },
  { id: "lookup",   label: "查询工具" },
]

// ─── Static tools (have dedicated page files) ─────────────────────────────────

const STATIC_TOOLS: SidebarTool[] = [
  { slug: "diagnose",        name: "一键诊断",      description: "并发检测 DNS / HTTP / Ping / SSL",  subcategory: "probe"    },
  { slug: "http",            name: "HTTP 拨测",      description: "HTTP/HTTPS 请求响应检测",            subcategory: "probe"    },
  { slug: "ping",            name: "多地 Ping",      description: "全球节点延迟和丢包测试",              subcategory: "probe"    },
  { slug: "tcping",          name: "TCPing 测试",    description: "TCP 端口连通性检测",                 subcategory: "probe"    },
  { slug: "dns",             name: "DNS 解析",       description: "域名 DNS 记录全类型查询",            subcategory: "probe"    },
  { slug: "traceroute",      name: "路由追踪",       description: "网络路径和跳点延迟追踪",              subcategory: "probe"    },
  { slug: "json-formatter",  name: "JSON 格式化",    description: "JSON 美化、压缩和校验",              subcategory: "convert"  },
  { slug: "base64",          name: "Base64 编解码",  description: "Base64 编码与解码转换",              subcategory: "convert"  },
  { slug: "timestamp",       name: "时间戳转换",     description: "Unix 时间戳与日期时间互转",           subcategory: "convert"  },
  { slug: "jwt-decoder",     name: "JWT 解码",       description: "JWT Token 解析查看",                subcategory: "convert"  },
  { slug: "hash",            name: "哈希计算",       description: "MD5 / SHA1 / SHA256 哈希",          subcategory: "generate" },
  { slug: "qrcode",          name: "二维码生成",     description: "文本 / URL 生成二维码图片",           subcategory: "generate" },
  { slug: "regex-tester",    name: "正则测试",       description: "正则表达式在线测试与调试",            subcategory: "lookup"   },
  { slug: "cron-parser",     name: "Cron 解析",      description: "Cron 表达式解析与下次执行时间",       subcategory: "lookup"   },
  { slug: "cidr-calculator", name: "CIDR 计算",      description: "IP 子网 / CIDR 范围计算",           subcategory: "lookup"   },
  { slug: "ipv6-converter",  name: "IPv6 转换",      description: "IPv6 格式检测与扩展/压缩转换",       subcategory: "lookup"   },
]

// ─── Subcategory map for dynamic tools ───────────────────────────────────────

const SUBCATEGORY_MAP: Record<string, ToolSubcategory> = {
  // probe
  ssl: "probe", whois: "probe", icp: "probe", ip: "probe", tcp: "probe",
  mtr: "probe", smtp: "probe", rdns: "probe", asn: "probe", mx: "probe",
  spf: "probe", dmarc: "probe", ntp: "probe", dkim: "probe", bgp: "probe",
  // text
  "word-counter": "text", "line-sort": "text", "duplicate-remover": "text",
  "text-case": "text", "html-encode": "text", "escape-html": "text",
  "text-stats": "text", diff: "text", markdown: "text",
  // convert
  "url-encode": "convert", unicode: "convert", "jwt-decode": "convert",
  "number-convert": "convert", "json-to-yaml": "convert", "yaml-formatter": "convert",
  "xml-formatter": "convert", "url-parser": "convert", "user-agent": "convert",
  "number-format": "convert",
  // generate
  "password-gen": "generate", "uuid-gen": "generate", lorem: "generate",
  "chmod-calc": "generate", "sort-json": "generate", "color-picker": "generate",
  "image-base64": "generate",
  // lookup
  regex: "lookup", "cron-viz": "lookup", "cidr-calc": "lookup", "ipv6-check": "lookup",
  "http-status": "lookup", "mime-type": "lookup", timezone: "lookup",
  "date-calc": "lookup", "csv-formatter": "lookup",
}

// ─── Build unified tool list ──────────────────────────────────────────────────

const STATIC_SLUGS = new Set(STATIC_TOOLS.map(t => t.slug))

const ALL_SIDEBAR_TOOLS: SidebarTool[] = [
  ...STATIC_TOOLS,
  ...ALL_TOOLS
    .filter(t => !STATIC_SLUGS.has(t.slug))
    .map(t => ({
      slug: t.slug,
      name: t.name,
      description: t.description,
      subcategory: SUBCATEGORY_MAP[t.slug] ?? (t.category === "probe" ? "probe" : "lookup") as ToolSubcategory,
    })),
]

const COUNTS: Record<CategoryId, number> = {
  all:      ALL_SIDEBAR_TOOLS.length,
  probe:    ALL_SIDEBAR_TOOLS.filter(t => t.subcategory === "probe").length,
  convert:  ALL_SIDEBAR_TOOLS.filter(t => t.subcategory === "convert").length,
  text:     ALL_SIDEBAR_TOOLS.filter(t => t.subcategory === "text").length,
  generate: ALL_SIDEBAR_TOOLS.filter(t => t.subcategory === "generate").length,
  lookup:   ALL_SIDEBAR_TOOLS.filter(t => t.subcategory === "lookup").length,
}

function slugToCategory(slug: string): CategoryId {
  return ALL_SIDEBAR_TOOLS.find(t => t.slug === slug)?.subcategory ?? "all"
}

// ─── Sidebar component ────────────────────────────────────────────────────────

interface SidebarProps {
  active: CategoryId
  onCategory: (id: CategoryId) => void
  search: string
  onSearch: (s: string) => void
  tools: SidebarTool[]
  currentSlug: string
  onNavigate?: () => void
}

function ToolsSidebar({ active, onCategory, search, onSearch, tools, currentSlug, onNavigate }: SidebarProps) {
  return (
    <div className="flex h-full w-full overflow-hidden">

      {/* ── Category column ──────────────────────────────────────────── */}
      <div className="flex w-[132px] shrink-0 flex-col border-r bg-muted/40">
        <div className="border-b px-3 py-[13px]">
          <p className="text-[10.5px] font-semibold uppercase tracking-widest text-muted-foreground/60">
            分类
          </p>
        </div>

        <nav className="flex-1 space-y-px overflow-y-auto p-2">
          {CATEGORIES.map(cat => {
            const isActive = active === cat.id
            return (
              <button
                key={cat.id}
                onClick={() => onCategory(cat.id)}
                className={cn(
                  "flex w-full items-center justify-between gap-1.5 rounded-md px-2.5 py-2 text-left text-[13px] font-medium transition-colors",
                  isActive
                    ? "bg-primary text-primary-foreground shadow-sm"
                    : "text-muted-foreground hover:bg-muted hover:text-foreground"
                )}
              >
                <span className="truncate">{cat.label}</span>
                <Badge
                  variant="secondary"
                  className={cn(
                    "h-[18px] shrink-0 px-1 text-[10px] font-normal tabular-nums",
                    isActive
                      ? "border-transparent bg-white/20 text-white"
                      : "border-transparent bg-muted-foreground/15 text-muted-foreground"
                  )}
                >
                  {COUNTS[cat.id]}
                </Badge>
              </button>
            )
          })}
        </nav>

        <div className="border-t px-3 py-2.5">
          <p className="text-[10px] tabular-nums text-muted-foreground/50">
            {ALL_SIDEBAR_TOOLS.length} 个工具
          </p>
        </div>
      </div>

      {/* ── Tool list column ─────────────────────────────────────────── */}
      <div className="flex min-w-0 flex-1 flex-col">
        {/* Search */}
        <div className="shrink-0 border-b px-3 py-2.5">
          <div className="relative">
            <Search className="pointer-events-none absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground/60" />
            <Input
              placeholder="搜索工具名称或描述…"
              className="h-8 border-0 bg-muted/50 pl-8 text-[13px] ring-0 placeholder:text-muted-foreground/50 focus-visible:bg-background focus-visible:ring-1"
              value={search}
              onChange={e => onSearch(e.target.value)}
            />
          </div>
        </div>

        {/* List */}
        <ScrollArea className="flex-1">
          <div className="p-1.5">
            {tools.length === 0 ? (
              <div className="flex flex-col items-center py-14 text-center">
                <p className="text-sm text-muted-foreground">无匹配工具</p>
                <p className="mt-1 text-xs text-muted-foreground/50">换个关键词试试</p>
              </div>
            ) : (
              tools.map(tool => {
                const isActive = currentSlug === tool.slug
                return (
                  <Link
                    key={tool.slug}
                    href={`/tools/${tool.slug}` as any}
                    onClick={onNavigate}
                    className={cn(
                      "relative flex flex-col gap-0.5 rounded-md px-3 py-2.5 transition-colors",
                      isActive ? "bg-accent" : "hover:bg-muted/60"
                    )}
                  >
                    {isActive && (
                      <span className="absolute inset-y-1.5 left-0.5 w-[3px] rounded-full bg-primary" />
                    )}
                    <span className={cn(
                      "truncate text-[13px] font-medium leading-snug",
                      isActive ? "text-accent-foreground" : "text-foreground"
                    )}>
                      {tool.name}
                    </span>
                    <span className="truncate text-[11.5px] leading-snug text-muted-foreground">
                      {tool.description}
                    </span>
                  </Link>
                )
              })
            )}
          </div>
        </ScrollArea>
      </div>
    </div>
  )
}

// ─── Layout ───────────────────────────────────────────────────────────────────

export default function ToolsLayout({ children }: { children: React.ReactNode }) {
  const pathname = usePathname()
  const currentSlug = pathname.split("/tools/")[1]?.split("/")[0] ?? ""

  const [active, setActive] = useState<CategoryId>(() =>
    currentSlug ? slugToCategory(currentSlug) : "all"
  )
  const [search, setSearch]     = useState("")
  const [sheetOpen, setSheetOpen] = useState(false)

  // Keep category in sync when navigating via browser back/forward
  useEffect(() => {
    if (currentSlug) setActive(slugToCategory(currentSlug))
  }, [currentSlug])

  const filteredTools = useMemo(() => {
    const q = search.toLowerCase().trim()
    return ALL_SIDEBAR_TOOLS.filter(t => {
      const matchCat = active === "all" || t.subcategory === active
      const matchQ   = !q || t.name.toLowerCase().includes(q) || t.description.toLowerCase().includes(q)
      return matchCat && matchQ
    })
  }, [active, search])

  const handleCategory = (id: CategoryId) => {
    setActive(id)
    setSearch("")
    setSheetOpen(false)
  }

  const sidebarProps: SidebarProps = {
    active,
    onCategory: handleCategory,
    search,
    onSearch: setSearch,
    tools: filteredTools,
    currentSlug,
  }

  const activeTool = ALL_SIDEBAR_TOOLS.find(t => t.slug === currentSlug)

  return (
    // Nav height ≈ 53px (py-3 × 2 + 28px content + 1px border)
    <div className="flex h-[calc(100dvh-53px)] overflow-hidden border-t">

      {/* Desktop sidebar (≥ md) */}
      <aside className="hidden w-[340px] shrink-0 md:flex">
        <ToolsSidebar {...sidebarProps} />
      </aside>

      {/* Mobile sheet */}
      <Sheet open={sheetOpen} onOpenChange={setSheetOpen}>
        <SheetContent side="left" className="flex w-[90vw] max-w-[340px] flex-col p-0">
          <SheetTitle className="sr-only">工具导航</SheetTitle>
          <div className="flex-1 overflow-hidden">
            <ToolsSidebar {...sidebarProps} onNavigate={() => setSheetOpen(false)} />
          </div>
        </SheetContent>
      </Sheet>

      {/* ── Right: Tool operation area ────────────────────────────────── */}
      <main className="flex min-w-0 flex-1 flex-col overflow-hidden border-l">

        {/* Mobile toolbar */}
        <div className="flex shrink-0 items-center gap-2 border-b px-3 py-2 md:hidden">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setSheetOpen(true)}
            className="-ml-1 h-8 gap-1.5 px-2 text-[13px]"
          >
            <Menu className="h-[15px] w-[15px]" />
            工具列表
          </Button>
          {activeTool && (
            <>
              <Separator orientation="vertical" className="h-4" />
              <span className="truncate text-[13px] text-muted-foreground">
                {activeTool.name}
              </span>
            </>
          )}
        </div>

        {/*
          ── Top banner / announcement zone ──────────────────────────────
          取消注释并填入内容即可启用顶部广告/公告栏：
          <div className="shrink-0 border-b bg-muted/40 px-6 py-2.5 text-sm text-center text-muted-foreground">
            公告内容 / 广告位
          </div>
        */}

        <ScrollArea className="flex-1">
          {children}
        </ScrollArea>

        {/*
          ── Bottom banner zone ───────────────────────────────────────────
          <div className="shrink-0 border-t bg-muted/30 px-6 py-2.5 text-sm text-center text-muted-foreground">
            底部广告位
          </div>
        */}
      </main>
    </div>
  )
}
