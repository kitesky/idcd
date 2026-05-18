"use client"

import { useState, useRef, useCallback, useEffect } from "react"
import Link from "next/link"
import { usePathname, useRouter, useSearchParams } from "next/navigation"
import { ChevronDown, Search, Globe, X, Menu, BadgeCheck, Bell, CreditCard, LogOut, Sparkles, LayoutDashboard, Sun, Moon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar"
import { locales as registryLocales, defaultLocale, isSupported, type Locale } from "@/i18n/registry"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
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
import { API_CREDENTIALS_POLICY } from "@/lib/api"
import { useTranslations } from "next-intl"
import { useTheme } from "next-themes"

// ── Mega menu data ─────────────────────────────────────────────────────────

function buildToolsMenu(t: ReturnType<typeof useTranslations<"nav">>, p: string) {
  return {
    categories: [
      {
        label: t("toolCategories.probe"),
        groups: [
          {
            title: t("toolGroups.network"),
            items: [
              { name: t("tools.ping"), href: `${p}/tools/ping` },
              { name: t("tools.tcp"), href: `${p}/tools/tcp` },
              { name: t("tools.tcping"), href: `${p}/tools/tcping` },
              { name: t("tools.traceroute"), href: `${p}/tools/traceroute` },
              { name: t("tools.mtr"), href: `${p}/tools/mtr` },
              { name: t("tools.speedtest"), href: `${p}/tools/speedtest` },
            ],
          },
          {
            title: t("toolGroups.domain"),
            items: [
              { name: t("tools.ssl"), href: `${p}/tools/ssl` },
              { name: t("tools.whois"), href: `${p}/tools/whois` },
              { name: t("tools.icp"), href: `${p}/tools/icp` },
              { name: t("tools.dns"), href: `${p}/tools/dns` },
              { name: t("tools.http"), href: `${p}/tools/http` },
            ],
          },
          {
            title: t("toolGroups.email"),
            items: [
              { name: t("tools.mx"), href: `${p}/tools/mx` },
              { name: t("tools.spf"), href: `${p}/tools/spf` },
              { name: t("tools.dmarc"), href: `${p}/tools/dmarc` },
              { name: t("tools.dkim"), href: `${p}/tools/dkim` },
              { name: t("tools.smtp"), href: `${p}/tools/smtp` },
              { name: t("tools.rdns"), href: `${p}/tools/rdns` },
              { name: t("tools.ntp"), href: `${p}/tools/ntp` },
            ],
          },
          {
            title: t("toolGroups.ip"),
            items: [
              { name: t("tools.ip-geo"), href: `${p}/tools/ip` },
              { name: t("tools.asn"), href: `${p}/tools/asn` },
              { name: t("tools.bgp"), href: `${p}/tools/bgp` },
            ],
          },
        ],
        featured: [
          { name: t("tools.ping"), desc: t("featuredProbeDesc.ping"), href: `${p}/tools/ping` },
          { name: t("tools.ssl"), desc: t("featuredProbeDesc.ssl"), href: `${p}/tools/ssl` },
          { name: t("tools.dns"), desc: t("featuredProbeDesc.dns"), href: `${p}/tools/dns` },
        ],
      },
      {
        label: t("toolCategories.utility"),
        groups: [
          {
            title: t("toolGroups.format"),
            items: [
              { name: t("tools.json-formatter"), href: `${p}/tools/json-formatter` },
              { name: t("tools.yaml-formatter"), href: `${p}/tools/yaml-formatter` },
              { name: t("tools.xml-formatter"), href: `${p}/tools/xml-formatter` },
              { name: t("tools.csv-formatter"), href: `${p}/tools/csv-formatter` },
            ],
          },
          {
            title: t("toolGroups.encode"),
            items: [
              { name: t("tools.base64"), href: `${p}/tools/base64` },
              { name: t("tools.url-encode"), href: `${p}/tools/url-encode` },
              { name: t("tools.hash"), href: `${p}/tools/hash` },
              { name: t("tools.html-encode"), href: `${p}/tools/html-encode` },
            ],
          },
          {
            title: t("toolGroups.text"),
            items: [
              { name: t("tools.regex-tester"), href: `${p}/tools/regex-tester` },
              { name: t("tools.diff"), href: `${p}/tools/diff` },
              { name: t("tools.word-counter"), href: `${p}/tools/word-counter` },
              { name: t("tools.markdown"), href: `${p}/tools/markdown` },
              { name: t("tools.line-sort"), href: `${p}/tools/line-sort` },
            ],
          },
          {
            title: t("toolGroups.dev"),
            items: [
              { name: t("tools.jwt-decoder"), href: `${p}/tools/jwt-decoder` },
              { name: t("tools.cidr-calculator"), href: `${p}/tools/cidr-calculator` },
              { name: t("tools.timestamp"), href: `${p}/tools/timestamp` },
              { name: t("tools.ipv6-converter"), href: `${p}/tools/ipv6-converter` },
              { name: t("tools.cron-parser"), href: `${p}/tools/cron-parser` },
              { name: t("tools.qrcode"), href: `${p}/tools/qrcode` },
            ],
          },
        ],
        featured: [
          { name: t("tools.json-formatter"), desc: t("featuredUtilityDesc.json"), href: `${p}/tools/json-formatter` },
          { name: t("tools.regex-tester"), desc: t("featuredUtilityDesc.regex"), href: `${p}/tools/regex-tester` },
          { name: t("tools.base64"), desc: t("featuredUtilityDesc.base64"), href: `${p}/tools/base64` },
        ],
      },
    ],
  }
}

// ── ThemeToggle ─────────────────────────────────────────────────────────────

function ThemeToggle() {
  const t = useTranslations("nav")
  const { resolvedTheme, setTheme } = useTheme()
  return (
    <Button
      variant="ghost"
      size="icon"
      onClick={() => setTheme(resolvedTheme === "dark" ? "light" : "dark")}
      aria-label={t("theme.toggle")}
      className="relative h-8 w-8 text-muted-foreground hover:text-foreground"
    >
      <Sun className="h-4 w-4 rotate-0 scale-100 transition-all dark:-rotate-90 dark:scale-0" />
      <Moon className="absolute h-4 w-4 rotate-90 scale-0 transition-all dark:rotate-0 dark:scale-100" />
    </Button>
  )
}

// ── NavUserMenu ─────────────────────────────────────────────────────────────

interface UserProfile {
  email: string
  display_name?: string | null
  avatar_url?: string | null
  plan?: string
}

function NavUserMenu({ mobile = false }: { mobile?: boolean }) {
  const t = useTranslations("nav")
  const pathname = usePathname()
  const p = pathname?.startsWith("/en") ? "/en" : ""
  const [user, setUser] = useState<UserProfile | null | undefined>(undefined) // undefined = loading

  useEffect(() => {
    const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080"
    fetch(`${API_BASE}/v1/account/profile`, { credentials: API_CREDENTIALS_POLICY })
      .then(res => {
        if (!res.ok) { setUser(null); return null }
        return res.json()
      })
      .then(body => {
        if (!body) return
        const profile = body?.data ?? body
        setUser({
          email: profile.email ?? "",
          display_name: profile.display_name ?? null,
          avatar_url: profile.avatar_url ?? null,
          plan: profile.plan ?? "Free",
        })
      })
      .catch(() => setUser(null))
  }, [])

  // Loading: show skeleton placeholder
  if (user === undefined) {
    return <div className="h-8 w-8 rounded-full bg-muted animate-pulse" />
  }

  // Not logged in
  if (user === null) {
    if (mobile) {
      return (
        <div className="flex flex-col gap-2">
          <Button variant="outline" className="w-full" asChild>
            <a href="/auth/login">{t("auth.login")}</a>
          </Button>
          <Button className="w-full" asChild>
            <a href="/auth/register">{t("auth.register")}</a>
          </Button>
        </div>
      )
    }
    return (
      <div className="flex items-center gap-2">
        <Button variant="ghost" size="sm" className="h-8 px-3 text-sm" asChild>
          <a href="/auth/login">{t("auth.login")}</a>
        </Button>
        <Button size="sm" className="h-8 px-4 text-sm" asChild>
          <a href="/auth/register">{t("auth.register")}</a>
        </Button>
      </div>
    )
  }

  // Logged in
  const initial = (user.display_name ?? user.email).charAt(0).toUpperCase()
  const isFree = !user.plan || user.plan === "Free"

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button className="flex items-center gap-2 rounded-full outline-none focus-visible:ring-2 focus-visible:ring-ring">
          <Avatar className="h-8 w-8 cursor-pointer ring-2 ring-transparent hover:ring-primary/40 transition-all">
            <AvatarImage src={user.avatar_url ?? undefined} alt={user.display_name ?? user.email} />
            <AvatarFallback className="bg-primary/10 text-primary text-sm font-semibold">
              {initial}
            </AvatarFallback>
          </Avatar>
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent className="w-56 rounded-lg" align="end" sideOffset={8}>
        <DropdownMenuLabel className="p-0 font-normal">
          <div className="flex items-center gap-2 px-2 py-2">
            <Avatar className="h-9 w-9 rounded-lg">
              <AvatarImage src={user.avatar_url ?? undefined} alt={user.display_name ?? user.email} />
              <AvatarFallback className="rounded-lg bg-primary/10 text-primary text-sm font-semibold">
                {initial}
              </AvatarFallback>
            </Avatar>
            <div className="grid flex-1 text-left text-sm leading-tight">
              <span className="truncate font-semibold">{user.display_name ?? user.email}</span>
              <span className="truncate text-xs text-muted-foreground">{user.email}</span>
            </div>
          </div>
        </DropdownMenuLabel>
        <DropdownMenuSeparator />
        {isFree && (
          <>
            <DropdownMenuGroup>
              <DropdownMenuItem asChild>
                <a href={`${p}/pricing`}>
                  <Sparkles className="text-primary" />
                  {t("auth.upgrade")}
                </a>
              </DropdownMenuItem>
            </DropdownMenuGroup>
            <DropdownMenuSeparator />
          </>
        )}
        <DropdownMenuGroup>
          <DropdownMenuItem asChild>
            <a href="/app/dashboard">
              <LayoutDashboard />
              {t("auth.dashboard")}
            </a>
          </DropdownMenuItem>
          <DropdownMenuItem asChild>
            <a href="/app/settings/account">
              <BadgeCheck />
              {t("auth.account")}
            </a>
          </DropdownMenuItem>
          <DropdownMenuItem asChild>
            <a href="/app/billing">
              <CreditCard />
              {t("auth.billing")}
            </a>
          </DropdownMenuItem>
          <DropdownMenuItem asChild>
            <a href="/app/alerts/channels">
              <Bell />
              {t("auth.notifications")}
            </a>
          </DropdownMenuItem>
        </DropdownMenuGroup>
        <DropdownMenuSeparator />
        <DropdownMenuItem asChild className="text-destructive focus:text-destructive">
          <a href="/auth/logout">
            <LogOut />
            {t("auth.logout")}
          </a>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

// ── LangToggle ──────────────────────────────────────────────────────────────

// Locale codes that carry a URL prefix (= every locale except the default).
const PREFIX_CODES = registryLocales
  .map(l => l.code)
  .filter(c => c !== defaultLocale)

/**
 * Detect the active locale from a URL path. Mirrors `matchLocalePrefix` in
 * proxy.ts so client-side state stays in sync with the middleware.
 */
function detectLocaleFromPath(pathname: string | null): Locale {
  if (!pathname) return defaultLocale
  for (const code of PREFIX_CODES) {
    if (pathname === `/${code}` || pathname.startsWith(`/${code}/`)) {
      return code
    }
  }
  return defaultLocale
}

/**
 * Strip any leading locale prefix from a pathname, leaving a path that starts
 * with `/`. Used when computing the destination URL for a locale switch.
 */
function stripLocalePrefix(pathname: string): string {
  for (const code of PREFIX_CODES) {
    if (pathname === `/${code}`) return "/"
    if (pathname.startsWith(`/${code}/`)) return pathname.slice(code.length + 1)
  }
  return pathname || "/"
}

/**
 * Build the new URL when switching to `target` locale. Preserves the rest of
 * the path; the caller is responsible for query / hash.
 */
function buildLocaleHref(pathname: string, target: Locale): string {
  const bare = stripLocalePrefix(pathname)
  if (target === defaultLocale) return bare
  return bare === "/" ? `/${target}` : `/${target}${bare}`
}

function LangToggle() {
  const t = useTranslations("nav")
  const pathname = usePathname() ?? "/"
  const searchParams = useSearchParams()
  const router = useRouter()

  const currentLocale = detectLocaleFromPath(pathname)

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

  const switchLocale = (target: Locale) => {
    setOpen(false)
    if (!isSupported(target) || target === currentLocale) return

    // Persist the choice so subsequent requests (incl. authenticated areas
    // without URL prefixes) keep the user's preferred language. Canonical
    // cookie is `idcd_locale`; we also clear the legacy `locale` cookie so
    // sessions converge on the new name.
    // eslint-disable-next-line react-hooks/immutability -- document.cookie 是浏览器 API，事件处理器内写入合法
    document.cookie = `idcd_locale=${target};path=/;max-age=31536000;samesite=lax`
    // eslint-disable-next-line react-hooks/immutability -- 清理旧 locale cookie
    document.cookie = "locale=;path=/;max-age=0"

    const isAuthArea = /^\/(app|auth|admin)(\/|$)/.test(pathname)

    if (isAuthArea) {
      // Authenticated areas don't carry a URL prefix; just refresh so the
      // server re-renders with the new locale (read from cookie).
      router.refresh()
      return
    }

    // Public pages: keep path + query + hash, switch the locale prefix.
    const qs = searchParams?.toString() ?? ""
    const hash = typeof window !== "undefined" ? window.location.hash : ""
    const target_path = buildLocaleHref(pathname, target)
    const href = `${target_path}${qs ? `?${qs}` : ""}${hash}`

    router.replace(href)
  }

  const currentEntry =
    registryLocales.find(l => l.code === currentLocale) ??
    registryLocales.find(l => l.code === defaultLocale)!

  return (
    <div ref={containerRef} className="relative">
      <button
        onClick={() => setOpen(v => !v)}
        className="flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground transition-colors"
      >
        <Globe className="h-3.5 w-3.5" />
        <span>{currentEntry.nativeLabel}</span>
        <ChevronDown className={cn(
          "h-3 w-3 transition-transform duration-150",
          open && "rotate-180"
        )} />
      </button>

      {open && (
        <div className="absolute top-full right-0 mt-1.5 w-48 bg-popover border rounded-lg shadow-lg py-2 z-50">
          <p className="text-xs text-muted-foreground px-4 pt-1 pb-1.5">{t("locale.label")}</p>
          {registryLocales.map(l => (
            <button
              key={l.code}
              onMouseDown={() => switchLocale(l.code)}
              className={cn(
                "block w-full text-left px-4 py-2 text-sm transition-colors",
                l.code === currentLocale
                  ? "text-primary bg-muted/60"
                  : "text-foreground hover:bg-muted"
              )}
            >
              {l.nativeLabel}
              <span className="text-muted-foreground ml-1.5 text-xs uppercase">– {l.code}</span>
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

  const tTools = useTranslations("tools")
  const lowerQuery = query.trim().toLowerCase()
  const results = lowerQuery
    ? ALL_TOOLS.filter(tool => {
        const title = String(tTools(`${tool.slug}.title` as never) ?? "").toLowerCase()
        return title.includes(lowerQuery) || tool.slug.includes(lowerQuery)
      }).slice(0, 8)
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
              {tTools(`${tool.slug}.title` as never)}
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
  p: string
}

function MegaMenuPanel({ categories, p }: MegaMenuProps) {
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
              href={`${p}/tools`}
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

function MobileSheet({ navItems, p }: { navItems: ReturnType<typeof buildNavItems>; p: string }) {
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
            <Link href="/" onClick={close} className="font-mono font-bold text-foreground text-lg">idcd</Link>
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
                        <a href={`${p}/tools`} onClick={close} className="mt-2 block text-xs text-primary hover:underline">
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
          <div className="flex items-center justify-between">
            <LangToggle />
            <ThemeToggle />
          </div>
          <div className="mt-2">
            <NavUserMenu mobile />
          </div>
        </div>
      </SheetContent>
    </Sheet>
  )
}

// ── Nav items builder ────────────────────────────────────────────────────────

function buildNavItems(t: ReturnType<typeof useTranslations<"nav">>, p: string) {
  const toolsMenu = buildToolsMenu(t, p)
  return [
    { name: t("links.tools"), mega: toolsMenu, href: undefined as string | undefined },
    { name: t("links.monitors"), mega: undefined as typeof toolsMenu | undefined, href: `${p}/monitors` },
    { name: t("links.freeSslCert"), mega: undefined, href: `${p}/cert` },
    { name: t("links.agent"), mega: undefined, href: `${p}/agent` },
    { name: t("links.docs"), mega: undefined, href: `${p}/docs/api` },
  ]
}

// ── Nav ─────────────────────────────────────────────────────────────────────

export function Nav() {
  const t = useTranslations("nav")
  const pathname = usePathname()
  const p = pathname?.startsWith("/en") ? "/en" : ""
  const navItems = buildNavItems(t, p)

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
        <a href={p || "/"} className="flex items-center gap-2 flex-shrink-0">
          <svg className="h-6 w-6 text-primary" viewBox="0 0 24 24" fill="currentColor">
            <path d="M3 12L12 3l9 9v9H3v-9z" opacity="0.2" />
            <path d="M3 12L12 3l9 9" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
          <span className="font-mono font-bold text-foreground text-lg tracking-tight">idcd</span>
        </a>

        {/* Desktop nav */}
        <div className="hidden md:flex md:items-center md:gap-0 ml-6 min-w-0 flex-1">
          {navItems.map((item) => (
            <div
              key={item.name}
              className="relative flex-shrink-0"
              onMouseEnter={() => item.mega ? handleMouseEnter(item.name) : undefined}
              onMouseLeave={item.mega ? handleMouseLeave : undefined}
            >
              {item.mega ? (
                <button
                  className={cn(
                    "flex items-center gap-0.5 px-3 py-1.5 text-sm font-medium rounded-md transition-colors whitespace-nowrap",
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
                  className="flex items-center px-3 py-1.5 text-sm font-medium rounded-md text-foreground/80 hover:text-foreground hover:bg-muted/60 transition-colors whitespace-nowrap"
                >
                  {item.name}
                </a>
              )}
            </div>
          ))}
        </div>

        {/* Desktop right */}
        <div className="hidden md:flex md:items-center md:gap-3 ml-auto flex-shrink-0">
          <NavSearch />
          <ThemeToggle />
          <LangToggle />
          <div className="pl-2 border-l">
            <NavUserMenu />
          </div>
        </div>

        {/* Mobile menu */}
        <MobileSheet navItems={navItems} p={p} />
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
            <MegaMenuPanel categories={item.mega.categories} p={p} />
          </div>
        )
      })}

    </header>
  )
}
