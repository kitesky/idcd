"use client"

import { useEffect, useState } from "react"
import Link from "next/link"
import { usePathname, useRouter } from "next/navigation"
import {
  BarChart3,
  Bell,
  ChevronDown,
  CreditCard,
  Globe,
  LayoutDashboard,
  LogOut,
  Menu,
  Settings,
  X,
} from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { cn } from "@/lib/utils"

// ─── Nav items ────────────────────────────────────────────────────────────────

const NAV_GROUPS = [
  {
    label: "监控",
    items: [
      { icon: LayoutDashboard, label: "监控列表", href: "/app/monitors" },
      { icon: Bell, label: "告警管理", href: "/app/alerts" },
    ],
  },
  {
    label: "发布",
    items: [
      { icon: Globe, label: "状态页", href: "/app/status-pages" },
    ],
  },
  {
    label: "账号",
    items: [
      { icon: CreditCard, label: "订阅与计费", href: "/app/billing" },
      { icon: BarChart3, label: "用量统计", href: "/app/usage" },
      { icon: Settings, label: "个人设置", href: "/app/settings/profile" },
    ],
  },
] as const

// ─── Sidebar content (shared between desktop + mobile) ────────────────────────

function SidebarContent({
  pathname,
  onNavClick,
}: {
  pathname: string
  onNavClick?: () => void
}) {
  return (
    <nav className="flex flex-col gap-6 px-3 py-4" data-testid="sidebar-nav">
      {NAV_GROUPS.map((group) => (
        <div key={group.label}>
          <p className="mb-1 px-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
            {group.label}
          </p>
          <div className="flex flex-col gap-0.5">
            {group.items.map((item) => {
              const active = pathname.startsWith(item.href)
              return (
                <Button
                  key={item.href}
                  variant={active ? "secondary" : "ghost"}
                  className={cn(
                    "w-full justify-start gap-2 text-sm",
                    active && "font-medium"
                  )}
                  asChild
                  data-testid={`nav-item-${item.href.replace(/\//g, "-").replace(/^-/, "")}`}
                  data-active={active ? "true" : undefined}
                  onClick={onNavClick}
                >
                  <Link href={item.href}>
                    <item.icon className="h-4 w-4 shrink-0" />
                    {item.label}
                  </Link>
                </Button>
              )
            })}
          </div>
        </div>
      ))}
    </nav>
  )
}

// ─── App Shell Layout ──────────────────────────────────────────────────────────

export default function AppLayout({ children }: { children: React.ReactNode }) {
  const router = useRouter()
  const pathname = usePathname()

  const [authChecked, setAuthChecked] = useState(false)
  const [plan, setPlan] = useState<string>("Free")
  const [userEmail, setUserEmail] = useState<string>("user@example.com")
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false)

  // Auth guard + load mock data from localStorage
  useEffect(() => {
    if (typeof window === "undefined") return
    const token = localStorage.getItem("auth_token")
    if (!token) {
      router.replace("/auth/login")
      return
    }
    const savedPlan = localStorage.getItem("mock_plan") ?? "Free"
    setPlan(savedPlan)
    const savedEmail = localStorage.getItem("mock_email") ?? "user@example.com"
    setUserEmail(savedEmail)
    setAuthChecked(true)
  }, [router])

  // Close mobile menu on route change
  useEffect(() => {
    setMobileMenuOpen(false)
  }, [pathname])

  // Show skeleton shell while auth check runs — prevents CLS and content flash
  if (!authChecked) {
    return (
      <div className="flex min-h-screen flex-col bg-background">
        <header className="sticky top-0 z-50 flex h-14 items-center border-b bg-background/95 px-4">
          <Skeleton className="h-6 w-16" />
          <div className="flex-1" />
          <Skeleton className="h-8 w-24 rounded-full" />
        </header>
        <div className="flex flex-1">
          <aside className="hidden w-60 shrink-0 border-r md:block">
            <div className="flex flex-col gap-2 px-3 py-4">
              {Array.from({ length: 6 }).map((_, i) => (
                <Skeleton key={i} className="h-9 w-full rounded-md" />
              ))}
            </div>
          </aside>
          <main className="flex-1 p-6">
            <Skeleton className="h-8 w-48 mb-4" />
            <Skeleton className="h-64 w-full rounded-lg" />
          </main>
        </div>
      </div>
    )
  }

  const planVariant =
    plan === "Pro"
      ? "default"
      : plan === "Team" || plan === "Business"
        ? "success"
        : "secondary"

  return (
    <div className="flex min-h-screen flex-col bg-background">
      {/* ── Header ─────────────────────────────────────────────────────────── */}
      <header
        className="sticky top-0 z-50 flex h-14 items-center border-b bg-background/95 px-4 backdrop-blur supports-[backdrop-filter]:bg-background/60"
        data-testid="app-header"
      >
        {/* Mobile hamburger */}
        <Button
          variant="ghost"
          size="icon"
          className="mr-2 md:hidden"
          aria-label="打开菜单"
          data-testid="mobile-menu-button"
          onClick={() => setMobileMenuOpen((v) => !v)}
        >
          <Menu className="h-5 w-5" />
        </Button>

        {/* Logo */}
        <Link
          href="/"
          className="flex items-center gap-1.5 font-mono font-bold text-primary"
          data-testid="logo-link"
        >
          <span className="text-lg">idcd</span>
          <span className="h-1.5 w-1.5 rounded-full bg-primary" aria-hidden />
        </Link>

        {/* Spacer */}
        <div className="flex-1" />

        {/* Plan badge */}
        <Badge
          variant={planVariant as "default" | "secondary" | "destructive" | "outline"}
          className="mr-3"
          data-testid="plan-badge"
        >
          {plan}
        </Badge>

        {/* User menu */}
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              variant="ghost"
              size="sm"
              className="flex items-center gap-1.5"
              aria-label="用户菜单"
              data-testid="user-menu-trigger"
            >
              <div className="flex h-7 w-7 items-center justify-center rounded-full bg-primary/10 text-xs font-semibold text-primary">
                {userEmail.charAt(0).toUpperCase()}
              </div>
              <span className="hidden max-w-[140px] truncate text-sm sm:inline-block">
                {userEmail}
              </span>
              <ChevronDown className="h-3.5 w-3.5 text-muted-foreground" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-48" data-testid="user-dropdown">
            <DropdownMenuLabel className="text-xs font-normal text-muted-foreground truncate">
              {userEmail}
            </DropdownMenuLabel>
            <DropdownMenuSeparator />
            <DropdownMenuItem asChild>
              <Link href="/app/settings/profile" className="flex items-center gap-2 cursor-pointer">
                <Settings className="h-4 w-4" />
                设置
              </Link>
            </DropdownMenuItem>
            <DropdownMenuItem asChild>
              <Link href="/auth/logout" className="flex items-center gap-2 cursor-pointer text-destructive focus:text-destructive">
                <LogOut className="h-4 w-4" />
                退出
              </Link>
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </header>

      {/* ── Body: sidebar + main ────────────────────────────────────────────── */}
      <div className="flex flex-1">
        {/* Desktop sidebar */}
        <aside
          className="hidden w-60 shrink-0 border-r md:block"
          data-testid="desktop-sidebar"
        >
          <div className="sticky top-14 h-[calc(100vh-3.5rem)] overflow-y-auto">
            <SidebarContent pathname={pathname} />
          </div>
        </aside>

        {/* Mobile sidebar overlay */}
        {mobileMenuOpen && (
          <>
            {/* Backdrop */}
            <div
              className="fixed inset-0 z-40 bg-black/50 md:hidden"
              aria-hidden
              onClick={() => setMobileMenuOpen(false)}
              data-testid="mobile-overlay"
            />
            {/* Sheet panel */}
            <div
              className="fixed left-0 top-0 z-50 flex h-full w-64 flex-col bg-background shadow-xl md:hidden"
              data-testid="mobile-sidebar"
            >
              <div className="flex h-14 items-center justify-between border-b px-4">
                <Link
                  href="/"
                  className="font-mono font-bold text-primary"
                  onClick={() => setMobileMenuOpen(false)}
                >
                  idcd
                </Link>
                <Button
                  variant="ghost"
                  size="icon"
                  aria-label="关闭菜单"
                  data-testid="mobile-close-button"
                  onClick={() => setMobileMenuOpen(false)}
                >
                  <X className="h-5 w-5" />
                </Button>
              </div>
              <div className="flex-1 overflow-y-auto">
                <SidebarContent
                  pathname={pathname}
                  onNavClick={() => setMobileMenuOpen(false)}
                />
              </div>
            </div>
          </>
        )}

        {/* Main content */}
        <main className="flex-1 overflow-x-hidden p-6" data-testid="app-main">
          {children}
        </main>
      </div>
    </div>
  )
}
