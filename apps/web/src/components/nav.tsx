"use client"

import { useState, useRef, useCallback, useEffect } from "react"
import { usePathname, useRouter } from "next/navigation"
import { ChevronDown, Search, Globe, X, Menu } from "lucide-react"
import { Button } from "@/components/ui/button"
import { ALL_TOOLS } from "@/app/(public)/tools/tools-config"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet"
import { cn } from "@/lib/utils"
import { useTranslations } from "next-intl"

// ── Mega menu data ─────────────────────────────────────────────────────────

function buildToolsMenu(t: ReturnType<typeof useTranslations<"nav">>) {
  return {
    categories: [
      {
        label: t("toolCategories.probe"),
        groups: [
          {
            title: t("toolGroups.network"),
            items: [
              { name: t("tools.ping"), href: "/tools/ping" },
              { name: t("tools.tcp"), href: "/tools/tcp" },
              { name: t("tools.tcping"), href: "/tools/tcping" },
              { name: t("tools.traceroute"), href: "/tools/traceroute" },
              { name: t("tools.mtr"), href: "/tools/mtr" },
              { name: t("tools.speedtest"), href: "/tools/speedtest" },
            ],
          },
          {
            title: t("toolGroups.domain"),
            items: [
              { name: t("tools.ssl"), href: "/tools/ssl" },
              { name: t("tools.whois"), href: "/tools/whois" },
              { name: t("tools.icp"), href: "/tools/icp" },
              { name: t("tools.dns"), href: "/tools/dns" },
              { name: t("tools.http"), href: "/tools/http" },
            ],
          },
          {
            title: t("toolGroups.email"),
            items: [
              { name: t("tools.mx"), href: "/tools/mx" },
              { name: t("tools.spf"), href: "/tools/spf" },
              { name: t("tools.dmarc"), href: "/tools/dmarc" },
              { name: t("tools.dkim"), href: "/tools/dkim" },
              { name: t("tools.smtp"), href: "/tools/smtp" },
              { name: t("tools.rdns"), href: "/tools/rdns" },
              { name: t("tools.ntp"), href: "/tools/ntp" },
            ],
          },
          {
            title: t("toolGroups.ip"),
            items: [
              { name: t("tools.ip-geo"), href: "/tools/ip" },
              { name: t("tools.asn"), href: "/tools/asn" },
              { name: t("tools.bgp"), href: "/tools/bgp" },
            ],
          },
        ],
        featured: [
          { name: t("tools.ping"), desc: t("featuredProbeDesc.ping"), href: "/tools/ping" },
          { name: t("tools.ssl"), desc: t("featuredProbeDesc.ssl"), href: "/tools/ssl" },
          { name: t("tools.dns"), desc: t("featuredProbeDesc.dns"), href: "/tools/dns" },
        ],
      },
      {
        label: t("toolCategories.utility"),
        groups: [
          {
            title: t("toolGroups.format"),
            items: [
              { name: t("tools.json-formatter"), href: "/tools/json-formatter" },
              { name: t("tools.yaml-formatter"), href: "/tools/yaml-formatter" },
              { name: t("tools.xml-formatter"), href: "/tools/xml-formatter" },
              { name: t("tools.csv-formatter"), href: "/tools/csv-formatter" },
            ],
          },
          {
            title: t("toolGroups.encode"),
            items: [
              { name: t("tools.base64"), href: "/tools/base64" },
              { name: t("tools.url-encode"), href: "/tools/url-encode" },
              { name: t("tools.hash"), href: "/tools/hash" },
              { name: t("tools.html-encode"), href: "/tools/html-encode" },
            ],
          },
          {
            title: t("toolGroups.text"),
            items: [
              { name: t("tools.regex-tester"), href: "/tools/regex-tester" },
              { name: t("tools.diff"), href: "/tools/diff" },
              { name: t("tools.word-counter"), href: "/tools/word-counter" },
              { name: t("tools.markdown"), href: "/tools/markdown" },
              { name: t("tools.line-sort"), href: "/tools/line-sort" },
            ],
          },
          {
            title: t("toolGroups.dev"),
            items: [
              { name: t("tools.jwt-decoder"), href: "/tools/jwt-decoder" },
              { name: t("tools.cidr-calculator"), href: "/tools/cidr-calculator" },
              { name: t("tools.timestamp"), href: "/tools/timestamp" },
              { name: t("tools.ipv6-converter"), href: "/tools/ipv6-converter" },
              { name: t("tools.cron-parser"), href: "/tools/cron-parser" },
              { name: t("tools.qrcode"), href: "/tools/qrcode" },
            ],
          },
        ],
        featured: [
          { name: t("tools.json-formatter"), desc: t("featuredUtilityDesc.json"), href: "/tools/json-formatter" },
          { name: t("tools.regex-tester"), desc: t("featuredUtilityDesc.regex"), href: "/tools/regex-tester" },
          { name: t("tools.base64"), desc: t("featuredUtilityDesc.base64"), href: "/tools/base64" },
        ],
      },
    ],
  }
}

// ── LangToggle ──────────────────────────────────────────────────────────────

function LangToggle() {
  const t = useTranslations("nav")
  const pathname = usePathname()
  const router = useRouter()
  const isEn = pathname?.startsWith("/en") ?? false
  const currentCode = isEn ? "EN" : "ZH"
  const currentLabel = isEn ? t("locale.en") : t("locale.zh")

  const [open, setOpen] = useState(false)
  const containerRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return
    const handler = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    document.addEventListener("mousedown", handler)
    return () => document.removeEventListener("mousedown", handler)
  }, [open])

  const switchLocale = (code: "EN" | "ZH") => {
    setOpen(false)
    if (code === currentCode) return

    // Check if we're on a public page (not /app, /auth, /admin)
    const isPublicPage = !pathname?.match(/^\/(app|auth|admin)(\/|$)/)

    if (isPublicPage) {
      if (code === "EN") {
        // Add /en prefix
        const withoutEn = pathname?.replace(/^\/en/, "") || "/"
        router.push("/en" + withoutEn)
      } else {
        // Remove /en prefix
        const withoutEn = pathname?.replace(/^\/en/, "") || "/"
        router.push(withoutEn || "/")
      }
    } else {
      // App/Auth pages: set cookie and reload
      document.cookie = `locale=${code === "EN" ? "en" : "zh"};path=/;max-age=31536000`
      router.refresh()
    }
  }

  const langs = [
    { label: t("locale.zh"), code: "ZH" as const },
    { label: t("locale.en"), code: "EN" as const },
  ]

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
          <p className="text-xs text-muted-foreground px-4 pt-1 pb-1.5">{t("locale.label")}</p>
          {langs.map(l => (
            <button
              key={l.code}
              onMouseDown={() => switchLocale(l.code)}
              className={cn(
                "block w-full text-left px-4 py-2 text-sm transition-colors",
                l.code === currentCode
                  ? "text-primary bg-muted/60"
                  : "text-foreground hover:bg-muted"
              )}
            >
              {l.label}
              <span className="text-muted-foreground ml-1.5 text-xs">– {l.code}</span>
            </button>
          ))}
        </div>
      )}
    </div>
  )
}

// ── NavSearch ────────────────────────────────────────────────────────────────

const POPULAR_TOOLS = ALL_TOOLS.filter(t => t.category === "probe").slice(0, 5)

function NavSearch() {
  const t = useTranslations("nav")
  const [query, setQuery] = useState("")
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
      if (e.key === "Escape") { setOpen(false); setQuery("") }
    }
    document.addEventListener("mousedown", handler)
    document.addEventListener("keydown", keyHandler)
    return () => {
      document.removeEventListener("mousedown", handler)
      document.removeEventListener("keydown", keyHandler)
    }
  }, [open])

  const results = query.trim()
    ? ALL_TOOLS.filter(tool =>
        tool.name.includes(query) || tool.slug.includes(query.toLowerCase())
      ).slice(0, 8)
    : []

  const dropdownItems = results.length > 0 ? results : POPULAR_TOOLS
  const dropdownLabel = results.length > 0 ? t("search.results") : t("search.popular")

  return (
    <div ref={containerRef} className="relative">
      {/* Input */}
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
          placeholder={t("search.placeholder")}
          className="flex-1 min-w-0 bg-transparent outline-none placeholder:text-muted-foreground/60 text-sm"
        />
        {open && (
          <button
            onMouseDown={e => { e.preventDefault(); setOpen(false); setQuery("") }}
            className="text-muted-foreground hover:text-foreground transition-colors flex-shrink-0"
            aria-label={t("search.close")}
          >
            <X className="h-3.5 w-3.5" />
          </button>
        )}
      </div>

      {/* Dropdown */}
      {open && (
        <div className="absolute top-full right-0 mt-1.5 w-64 bg-popover border rounded-lg shadow-lg py-2 z-50">
          <p className="text-xs text-muted-foreground px-4 pt-1 pb-2">{dropdownLabel}</p>
          {dropdownItems.map(tool => (
            <a
              key={tool.slug}
              href={`/tools/${tool.slug}`}
              onMouseDown={() => setOpen(false)}
              className="block px-4 py-2 text-sm text-foreground hover:bg-muted transition-colors"
            >
              {tool.name}
            </a>
          ))}
        </div>
      )}
    </div>
  )
}

// ── MegaMenu panel ──────────────────────────────────────────────────────────

interface MegaMenuCategory {
  label: string
  groups: { title: string; items: { name: string; href: string }[] }[]
  featured: { name: string; desc: string; href: string }[]
}

interface MegaMenuProps {
  categories: MegaMenuCategory[]
}

function MegaMenuPanel({ categories }: MegaMenuProps) {
  const t = useTranslations("nav")
  const [activeCat, setActiveCat] = useState(0)
  const cat = categories[activeCat] ?? categories[0]

  if (!cat) return null

  return (
    <div className="absolute top-full left-0 right-0 z-50 bg-background border-b shadow-lg">
      <div className="mx-auto max-w-screen-xl px-6 py-6 flex gap-0">
        {/* Left: category list */}
        <div className="w-44 flex-shrink-0 flex flex-col gap-0.5 border-r pr-4 mr-6">
          {categories.map((c, i) => (
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
              {t("featured.label")}
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
              {t("featured.viewAll")}
            </a>
          </div>
        </div>
      </div>
    </div>
  )
}

// ── MobileSheet ─────────────────────────────────────────────────────────────

function MobileSheet({ navItems }: { navItems: ReturnType<typeof buildNavItems> }) {
  const t = useTranslations("nav")
  const [open, setOpen] = useState(false)
  const [expandedCat, setExpandedCat] = useState<string | null>(null)

  const close = () => setOpen(false)

  return (
    <Sheet open={open} onOpenChange={setOpen}>
      <SheetTrigger asChild>
        <Button variant="ghost" size="icon" className="md:hidden" aria-label={t("menu.open")}>
          <Menu className="h-5 w-5" />
        </Button>
      </SheetTrigger>
      <SheetContent side="left" className="w-72 p-0 flex flex-col">
        <SheetHeader className="border-b px-4 py-3 flex-shrink-0">
          <SheetTitle asChild>
            <a href="/" onClick={close} className="font-mono font-bold text-foreground text-lg">idcd</a>
          </SheetTitle>
          <SheetDescription className="sr-only">{t("menu.navigation")}</SheetDescription>
        </SheetHeader>

        {/* Scrollable nav body */}
        <div className="flex-1 overflow-y-auto py-2">
          {navItems.map((item) => {
            if (!item.mega) {
              return (
                <a
                  key={item.name}
                  href={item.href}
                  onClick={close}
                  className="flex items-center px-4 py-2.5 text-sm text-foreground/80 hover:text-foreground hover:bg-muted/60 transition-colors"
                >
                  {item.name}
                </a>
              )
            }
            // expandable mega item
            const isExpanded = expandedCat === item.name
            return (
              <div key={item.name}>
                <button
                  onClick={() => setExpandedCat(isExpanded ? null : item.name)}
                  className="flex items-center justify-between w-full px-4 py-2.5 text-sm text-foreground/80 hover:text-foreground hover:bg-muted/60 transition-colors"
                >
                  {item.name}
                  <ChevronDown className={cn("h-3.5 w-3.5 transition-transform duration-150", isExpanded && "rotate-180")} />
                </button>
                {isExpanded && (
                  <div className="bg-muted/30 border-y">
                    {item.mega.categories.map(cat => (
                      <div key={cat.label} className="px-4 py-3">
                        <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wider mb-2">{cat.label}</p>
                        <div className="grid grid-cols-2 gap-x-4 gap-y-1">
                          {cat.groups.flatMap(g => g.items).slice(0, 10).map(it => (
                            <a
                              key={it.href}
                              href={it.href}
                              onClick={close}
                              className="text-sm text-foreground/70 hover:text-primary transition-colors py-1 truncate"
                            >
                              {it.name}
                            </a>
                          ))}
                        </div>
                        <a href="/tools" onClick={close} className="mt-2 block text-xs text-primary hover:underline">
                          {t("featured.viewAll")}
                        </a>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            )
          })}
        </div>

        {/* Footer */}
        <div className="border-t p-4 flex flex-col gap-2 flex-shrink-0">
          <LangToggle />
          <Button variant="outline" className="w-full mt-2" asChild>
            <a href="/auth/login" onClick={close}>{t("auth.login")}</a>
          </Button>
          <Button className="w-full" asChild>
            <a href="/auth/register" onClick={close}>{t("auth.register")}</a>
          </Button>
        </div>
      </SheetContent>
    </Sheet>
  )
}

// ── Nav items builder ────────────────────────────────────────────────────────

function buildNavItems(t: ReturnType<typeof useTranslations<"nav">>) {
  const toolsMenu = buildToolsMenu(t)
  return [
    { name: t("links.tools"), mega: toolsMenu, href: undefined as string | undefined },
    { name: t("links.agent"), mega: undefined as typeof toolsMenu | undefined, href: "/agent" },
    { name: t("links.nodes"), mega: undefined, href: "/nodes" },
    { name: t("links.becomeNode"), mega: undefined, href: "/nodes/apply" },
    { name: t("links.pricing"), mega: undefined, href: "/pricing" },
    { name: t("links.docs"), mega: undefined, href: "/docs/api" },
  ]
}

// ── Nav ─────────────────────────────────────────────────────────────────────

export function Nav() {
  const t = useTranslations("nav")
  const navItems = buildNavItems(t)

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
              {t("auth.dashboard")}
            </a>
            <Button variant="ghost" size="sm" className="h-8 px-3 text-sm" asChild>
              <a href="/auth/login">{t("auth.login")}</a>
            </Button>
            <Button size="sm" className="h-8 px-4 text-sm" asChild>
              <a href="/auth/register">{t("auth.register")}</a>
            </Button>
          </div>
        </div>

        {/* Mobile menu */}
        <MobileSheet navItems={navItems} />
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
            <MegaMenuPanel categories={item.mega.categories} />
          </div>
        )
      })}

    </header>
  )
}
