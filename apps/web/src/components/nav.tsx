"use client"

import { useState, useRef, useCallback, useEffect } from "react"
import { usePathname } from "next/navigation"
import { ChevronDown, Search, Globe, X, Menu } from "lucide-react"
import { Button } from "@/components/ui/button"
import { ALL_TOOLS } from "@/app/(public)/tools/tools-config"
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet"
import { cn } from "@/lib/utils"

// ── Mega menu data ─────────────────────────────────────────────────────────

const toolsMenu = {
  categories: [
    {
      label: "拨测工具",
      groups: [
        {
          title: "网络连通",
          items: [
            { name: "Ping 测试", href: "/tools/ping" },
            { name: "TCP 端口", href: "/tools/tcp" },
            { name: "TCPing", href: "/tools/tcping" },
            { name: "Traceroute", href: "/tools/traceroute" },
            { name: "MTR 路由", href: "/tools/mtr" },
            { name: "网速测试", href: "/tools/speedtest" },
          ],
        },
        {
          title: "域名与证书",
          items: [
            { name: "SSL 证书检查", href: "/tools/ssl" },
            { name: "WHOIS 查询", href: "/tools/whois" },
            { name: "ICP 备案查询", href: "/tools/icp" },
            { name: "DNS 查询", href: "/tools/dns" },
            { name: "HTTP 检测", href: "/tools/http" },
          ],
        },
        {
          title: "邮件服务",
          items: [
            { name: "MX 记录查询", href: "/tools/mx" },
            { name: "SPF 记录查询", href: "/tools/spf" },
            { name: "DMARC 查询", href: "/tools/dmarc" },
            { name: "DKIM 密钥查询", href: "/tools/dkim" },
            { name: "SMTP 测试", href: "/tools/smtp" },
            { name: "反向 DNS 查询", href: "/tools/rdns" },
            { name: "NTP 服务测试", href: "/tools/ntp" },
          ],
        },
        {
          title: "IP 与路由",
          items: [
            { name: "IP 地理位置", href: "/tools/ip" },
            { name: "ASN 查询", href: "/tools/asn" },
            { name: "BGP 路由查询", href: "/tools/bgp" },
          ],
        },
      ],
      featured: [
        { name: "Ping 测试", desc: "全球节点网络拨测", href: "/tools/ping" },
        { name: "SSL 证书", desc: "证书有效期检查", href: "/tools/ssl" },
        { name: "DNS 查询", desc: "全球解析诊断", href: "/tools/dns" },
      ],
    },
    {
      label: "辅助工具",
      groups: [
        {
          title: "格式化",
          items: [
            { name: "JSON 格式化", href: "/tools/json-formatter" },
            { name: "YAML 格式化", href: "/tools/yaml-formatter" },
            { name: "XML 格式化", href: "/tools/xml-formatter" },
            { name: "CSV 格式化", href: "/tools/csv-formatter" },
          ],
        },
        {
          title: "编解码",
          items: [
            { name: "Base64 编解码", href: "/tools/base64" },
            { name: "URL 编解码", href: "/tools/url-encode" },
            { name: "Hash 计算", href: "/tools/hash" },
            { name: "HTML 实体编码", href: "/tools/html-encode" },
          ],
        },
        {
          title: "文本处理",
          items: [
            { name: "正则表达式", href: "/tools/regex-tester" },
            { name: "文本对比", href: "/tools/diff" },
            { name: "字数统计", href: "/tools/word-counter" },
            { name: "Markdown 预览", href: "/tools/markdown" },
            { name: "行排序", href: "/tools/line-sort" },
          ],
        },
        {
          title: "开发工具",
          items: [
            { name: "JWT 解码", href: "/tools/jwt-decoder" },
            { name: "CIDR 计算器", href: "/tools/cidr-calculator" },
            { name: "时间戳转换", href: "/tools/timestamp" },
            { name: "IPv6 检测", href: "/tools/ipv6-converter" },
            { name: "Cron 可视化", href: "/tools/cron-parser" },
            { name: "QR 码生成", href: "/tools/qrcode" },
          ],
        },
      ],
      featured: [
        { name: "JSON 格式化", desc: "格式化与压缩 JSON", href: "/tools/json-formatter" },
        { name: "正则表达式", desc: "实时高亮匹配", href: "/tools/regex-tester" },
        { name: "Base64", desc: "编解码与图片转换", href: "/tools/base64" },
      ],
    },
  ],
}

const navItems = [
  { name: "工具", mega: toolsMenu },
  { name: "节点", href: "/nodes" },
  { name: "成为节点", href: "/nodes/apply" },
  { name: "定价", href: "/pricing" },
  { name: "文档", href: "/docs/api" },
]

// ── LangToggle ──────────────────────────────────────────────────────────────

const LANGS = [
  { label: 'English', code: 'EN', href: '/en/' },
  { label: '简体中文', code: 'ZH', href: '/' },
  { label: '日本語', code: 'JP', href: '/ja/' },
  { label: '한국어', code: 'KO', href: '/ko/' },
]

function LangToggle() {
  const pathname = usePathname()
  const isEn = pathname?.startsWith('/en') ?? false
  const currentCode = isEn ? 'EN' : 'ZH'
  const currentLabel = isEn ? 'English' : '简体中文'

  const [open, setOpen] = useState(false)
  const containerRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return
    const handler = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [open])

  return (
    <div ref={containerRef} className="relative">
      <button
        onClick={() => setOpen(v => !v)}
        className="flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground transition-colors"
      >
        <Globe className="h-3.5 w-3.5" />
        <span>{currentLabel}</span>
        <ChevronDown className={cn(
          "h-3 w-3 transition-transform duration-150",
          open && "rotate-180"
        )} />
      </button>

      {open && (
        <div className="absolute top-full right-0 mt-1.5 w-48 bg-popover border rounded-lg shadow-lg py-2 z-50">
          <p className="text-xs text-muted-foreground px-4 pt-1 pb-1.5">语言</p>
          {LANGS.map(l => (
            <a
              key={l.code}
              href={l.href}
              onMouseDown={() => setOpen(false)}
              className={cn(
                "block px-4 py-2 text-sm transition-colors",
                l.code === currentCode
                  ? "text-primary bg-muted/60"
                  : "text-foreground hover:bg-muted"
              )}
            >
              {l.label}
              <span className="text-muted-foreground ml-1.5 text-xs">– {l.code}</span>
            </a>
          ))}
        </div>
      )}
    </div>
  )
}

// ── NavSearch ────────────────────────────────────────────────────────────────
// 静默态：带图标的输入框（始终可见）
// 激活态：输入框 focused + 右侧出现 × + 下方卡片下拉

const POPULAR_TOOLS = ALL_TOOLS.filter(t => t.category === 'probe').slice(0, 5)

function NavSearch() {
  const [query, setQuery] = useState('')
  const [open, setOpen] = useState(false)
  const containerRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    if (!open) return
    const handler = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    const keyHandler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') { setOpen(false); setQuery('') }
    }
    document.addEventListener('mousedown', handler)
    document.addEventListener('keydown', keyHandler)
    return () => {
      document.removeEventListener('mousedown', handler)
      document.removeEventListener('keydown', keyHandler)
    }
  }, [open])

  const results = query.trim()
    ? ALL_TOOLS.filter(t =>
        t.name.includes(query) || t.slug.includes(query.toLowerCase())
      ).slice(0, 8)
    : []

  const dropdownItems = results.length > 0 ? results : POPULAR_TOOLS
  const dropdownLabel = results.length > 0 ? '工具' : '热门工具'

  return (
    <div ref={containerRef} className="relative">
      {/* Input — 静默态始终显示 */}
      <div className={cn(
        "flex items-center gap-2 h-8 px-3 rounded-md border text-sm transition-colors w-52",
        open
          ? "border-primary bg-background"
          : "border-border bg-muted/40 hover:border-border/80"
      )}>
        <Search className="h-3.5 w-3.5 text-muted-foreground flex-shrink-0" />
        <input
          ref={inputRef}
          type="text"
          value={query}
          onChange={e => setQuery(e.target.value)}
          onFocus={() => setOpen(true)}
          placeholder="搜索工具..."
          className="flex-1 min-w-0 bg-transparent outline-none placeholder:text-muted-foreground/60 text-sm"
        />
        {open && (
          <button
            onMouseDown={e => { e.preventDefault(); setOpen(false); setQuery('') }}
            className="text-muted-foreground hover:text-foreground transition-colors flex-shrink-0"
            aria-label="关闭"
          >
            <X className="h-3.5 w-3.5" />
          </button>
        )}
      </div>

      {/* 下拉卡片 */}
      {open && (
        <div className="absolute top-full right-0 mt-1.5 w-64 bg-popover border rounded-lg shadow-lg py-2 z-50">
          <p className="text-xs text-muted-foreground px-4 pt-1 pb-2">{dropdownLabel}</p>
          {dropdownItems.map(t => (
            <a
              key={t.slug}
              href={`/tools/${t.slug}`}
              onMouseDown={() => setOpen(false)}
              className="block px-4 py-2 text-sm text-foreground hover:bg-muted transition-colors"
            >
              {t.name}
            </a>
          ))}
        </div>
      )}
    </div>
  )
}

// ── MegaMenu panel ──────────────────────────────────────────────────────────

interface MegaMenuProps {
  menu: typeof toolsMenu
}

function MegaMenuPanel({ menu }: MegaMenuProps) {
  const [activeCat, setActiveCat] = useState(0)
  const cat = menu.categories[activeCat] ?? menu.categories[0]

  if (!cat) return null

  return (
    <div className="absolute top-full left-0 right-0 z-50 bg-background border-b shadow-lg">
      <div className="mx-auto max-w-screen-xl px-6 py-6 flex gap-0">
        {/* Left: category list */}
        <div className="w-44 flex-shrink-0 flex flex-col gap-0.5 border-r pr-4 mr-6">
          {menu.categories.map((c, i) => (
            <button
              key={c.label}
              onMouseEnter={() => setActiveCat(i)}
              className={cn(
                "flex items-center justify-between text-sm px-3 py-2 rounded-md text-left w-full transition-colors",
                activeCat === i
                  ? "bg-muted text-foreground font-medium"
                  : "text-muted-foreground hover:text-foreground hover:bg-muted/50"
              )}
            >
              {c.label}
              {activeCat === i && (
                <svg className="h-3 w-3 text-primary ml-1" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M9 5l7 7-7 7" />
                </svg>
              )}
            </button>
          ))}
        </div>

        {/* Right: sub-group columns */}
        <div className="flex flex-1 gap-8">
          {cat.groups.map((g) => (
            <div key={g.title} className="min-w-[120px]">
              <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wider mb-2.5">
                {g.title}
              </p>
              <ul className="space-y-1.5">
                {g.items.map((item) => (
                  <li key={item.href}>
                    <a
                      href={item.href}
                      className="text-sm text-foreground/80 hover:text-primary transition-colors block"
                    >
                      {item.name}
                    </a>
                  </li>
                ))}
              </ul>
            </div>
          ))}

          {/* Featured column */}
          <div className="ml-auto pl-8 border-l min-w-[180px]">
            <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wider mb-2.5 flex items-center gap-1.5">
              热门工具
              <span className="inline-flex items-center rounded bg-primary/10 px-1.5 py-0.5 text-[10px] font-bold text-primary tracking-wide">
                HOT
              </span>
            </p>
            <ul className="space-y-3">
              {cat.featured.map((f) => (
                <li key={f.href}>
                  <a href={f.href} className="group block">
                    <p className="text-sm font-medium text-foreground group-hover:text-primary transition-colors">
                      {f.name}
                    </p>
                    <p className="text-xs text-muted-foreground mt-0.5">{f.desc}</p>
                  </a>
                </li>
              ))}
            </ul>
            <a
              href="/tools"
              className="mt-4 inline-flex items-center text-xs text-primary hover:underline"
            >
              查看全部工具 →
            </a>
          </div>
        </div>
      </div>
    </div>
  )
}

// ── Nav ─────────────────────────────────────────────────────────────────────

export function Nav() {
  const [openMenu, setOpenMenu] = useState<string | null>(null)
  const closeTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  const handleMouseEnter = useCallback((name: string) => {
    if (closeTimer.current) clearTimeout(closeTimer.current)
    setOpenMenu(name)
  }, [])

  const handleMouseLeave = useCallback(() => {
    closeTimer.current = setTimeout(() => setOpenMenu(null), 120)
  }, [])

  const handlePanelMouseEnter = useCallback(() => {
    if (closeTimer.current) clearTimeout(closeTimer.current)
  }, [])

  return (
    <header className="sticky top-0 z-50 w-full border-b bg-background/98 backdrop-blur supports-[backdrop-filter]:bg-background/95">
      {/* Main bar */}
      <nav className="mx-auto flex max-w-screen-xl items-center justify-between px-6 h-14">
        {/* Logo */}
        <a href="/" className="flex items-center gap-2 flex-shrink-0">
          <svg className="h-6 w-6 text-primary" viewBox="0 0 24 24" fill="currentColor">
            <path d="M3 12L12 3l9 9v9H3v-9z" opacity="0.2" />
            <path d="M3 12L12 3l9 9" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
          <span className="font-mono font-bold text-foreground text-lg tracking-tight">idcd</span>
        </a>

        {/* Desktop nav */}
        <div className="hidden md:flex md:items-center md:gap-0 ml-8">
          {navItems.map((item) => (
            <div
              key={item.name}
              className="relative"
              onMouseEnter={() => item.mega ? handleMouseEnter(item.name) : undefined}
              onMouseLeave={item.mega ? handleMouseLeave : undefined}
            >
              {item.mega ? (
                <button
                  className={cn(
                    "flex items-center gap-0.5 px-3.5 py-1.5 text-sm font-medium rounded-md transition-colors",
                    openMenu === item.name
                      ? "text-primary bg-muted"
                      : "text-foreground/80 hover:text-foreground hover:bg-muted/60"
                  )}
                >
                  {item.name}
                  <ChevronDown
                    className={cn(
                      "h-3.5 w-3.5 transition-transform duration-150",
                      openMenu === item.name && "rotate-180"
                    )}
                  />
                </button>
              ) : (
                <a
                  href={item.href}
                  className="flex items-center px-3.5 py-1.5 text-sm font-medium rounded-md text-foreground/80 hover:text-foreground hover:bg-muted/60 transition-colors"
                >
                  {item.name}
                </a>
              )}
            </div>
          ))}
        </div>

        {/* Desktop right */}
        <div className="hidden md:flex md:items-center md:gap-4 ml-auto">
          <NavSearch />
          <LangToggle />
          <div className="flex items-center gap-2 pl-2 border-l">
            <a href="/app" className="text-sm text-foreground/70 hover:text-foreground transition-colors">
              控制台
            </a>
            <Button variant="ghost" size="sm" className="h-8 px-3 text-sm" asChild>
              <a href="/auth/login">登录</a>
            </Button>
            <Button size="sm" className="h-8 px-4 text-sm" asChild>
              <a href="/auth/register">立即注册</a>
            </Button>
          </div>
        </div>

        {/* Mobile menu */}
        <Sheet>
          <SheetTrigger asChild>
            <Button variant="ghost" size="icon" className="md:hidden" aria-label="打开菜单">
              <Menu className="h-5 w-5" />
            </Button>
          </SheetTrigger>
          <SheetContent side="left" className="w-72 p-0">
            <SheetHeader className="border-b px-4 py-3">
              <SheetTitle asChild>
                <a href="/" className="font-mono font-bold text-foreground text-lg">idcd</a>
              </SheetTitle>
            </SheetHeader>
            <div className="flex flex-col gap-1 p-4">
              {navItems.map((item) => (
                <Button key={item.name} variant="ghost" className="justify-start" asChild={!item.mega}>
                  {item.mega ? (
                    <span>{item.name}</span>
                  ) : (
                    <a href={item.href}>{item.name}</a>
                  )}
                </Button>
              ))}
              <div className="mt-2 pt-2 border-t flex flex-col gap-2">
                <Button variant="ghost" className="justify-start" asChild>
                  <a href="/tools/ping">Ping 测试</a>
                </Button>
                <Button variant="ghost" className="justify-start" asChild>
                  <a href="/tools/ssl">SSL 证书检查</a>
                </Button>
                <Button variant="ghost" className="justify-start" asChild>
                  <a href="/tools/dns">DNS 查询</a>
                </Button>
                <a href="/tools" className="text-xs text-primary px-4 py-1.5 hover:underline">
                  查看全部工具 →
                </a>
              </div>
            </div>
            <div className="border-t p-4 flex flex-col gap-2">
              <LangToggle />
              <Button variant="outline" className="w-full mt-2" asChild>
                <a href="/auth/login">登录</a>
              </Button>
              <Button className="w-full" asChild>
                <a href="/auth/register">立即注册</a>
              </Button>
            </div>
          </SheetContent>
        </Sheet>
      </nav>

      {/* Mega menu panels (full-width, below nav) */}
      {navItems.map((item) => {
        if (!item.mega || openMenu !== item.name) return null
        return (
          <div
            key={item.name}
            onMouseEnter={handlePanelMouseEnter}
            onMouseLeave={handleMouseLeave}
          >
            <MegaMenuPanel menu={item.mega} />
          </div>
        )
      })}

    </header>
  )
}
