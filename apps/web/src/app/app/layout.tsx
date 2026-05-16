"use client"

import { useEffect, useState } from "react"
import Link from "next/link"
import { usePathname, useRouter } from "next/navigation"
import { SidebarProvider, SidebarInset, SidebarTrigger } from "@/components/ui/sidebar"
import { Skeleton } from "@/components/ui/skeleton"
import { Separator } from "@/components/ui/separator"
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb"
import { AppSidebar } from "@/components/layout/app-sidebar"

// ── 路由 → 面包屑标题映射 ─────────────────────────────────────
const ROUTE_TITLES: Record<string, string> = {
  "/app/dashboard":          "仪表盘",
  "/app/monitors":           "监控列表",
  "/app/monitors/new":       "创建监控",
  "/app/alerts":             "告警列表",
  "/app/alerts/channels":    "告警通道",
  "/app/alerts/policies":    "告警策略",
  "/app/alerts/groups":     "告警分组",
  "/app/oncall":             "On-Call 值班",
  "/app/incidents":          "故障记录",
  "/app/status-pages":       "状态页",
  "/app/reports":            "月度报告",
  "/app/billing":            "订阅与计费",
  "/app/usage":              "用量统计",
  "/app/referral":           "推荐计划",
  "/app/settings/profile":   "个人资料",
  "/app/settings/account":   "账户安全",
  "/app/settings/security":  "安全设置",
  "/app/settings/sessions":  "会话管理",
  "/app/settings/api-keys":  "API 密钥",
  "/app/settings/tokens":    "访问令牌",
  "/app/settings/team":      "团队管理",
  "/app/nodes":              "节点管理",
}

// ── Client 子组件（持有 state，处理 auth guard）────────────────
function AppShell({ children }: { children: React.ReactNode }) {
  const router = useRouter()
  const pathname = usePathname()

  const [authChecked, setAuthChecked] = useState(false)
  const [plan, setPlan]         = useState("Free")
  const [email, setEmail]       = useState("user@example.com")
  const [displayName, setDisplayName] = useState<string | null>(null)
  const [avatarUrl, setAvatarUrl] = useState<string | null>(null)

  useEffect(() => {
    // Verify session by calling the profile API — the HttpOnly access_token cookie
    // is sent automatically via credentials:"include" (set in apiRequest).
    fetch(`${process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080"}/v1/account/profile`, {
      credentials: "include",
    })
      .then((res) => {
        if (res.status === 401 || res.status === 403) {
          router.replace("/auth/login" as any)
          return null
        }
        return res.json()
      })
      .then((body) => {
        if (!body) return
        const profile = body?.data ?? body
        setEmail(profile.email ?? "user@example.com")
        setDisplayName(profile.display_name ?? null)
        setAvatarUrl(profile.avatar_url ?? null)
        setAuthChecked(true)
      })
      .catch(() => {
        router.replace("/auth/login" as any)
      })
  }, [router])

  if (!authChecked) {
    return (
      <div className="flex min-h-svh bg-background">
        <aside className="hidden w-60 shrink-0 border-r md:block">
          <div className="flex flex-col gap-2 p-4">
            {Array.from({ length: 8 }).map((_, i) => (
              <Skeleton key={i} className="h-9 w-full rounded-md" />
            ))}
          </div>
        </aside>
        <div className="flex flex-1 flex-col p-6 gap-4">
          <Skeleton className="h-8 w-48" />
          <Skeleton className="h-64 w-full rounded-lg" />
        </div>
      </div>
    )
  }

  // 当前页面标题（用于面包屑）
  const pageTitle =
    ROUTE_TITLES[pathname] ??
    Object.entries(ROUTE_TITLES).find(([k]) => pathname.startsWith(k + "/"))?.[1] ??
    "后台管理"
  const isSettings = pathname.startsWith("/app/settings")

  return (
    <SidebarProvider>
      <AppSidebar email={email} plan={plan} displayName={displayName} avatarUrl={avatarUrl} />
      <SidebarInset>
        {/* 顶部工具栏 */}
        <header className="flex h-12 shrink-0 items-center gap-2 border-b px-4">
          <SidebarTrigger className="-ml-1" data-testid="mobile-menu-button" aria-label="打开菜单" />
          <Separator orientation="vertical" className="mr-2 h-4" />
          <Breadcrumb>
            <BreadcrumbList>
              <BreadcrumbItem className="hidden md:block">
                <BreadcrumbLink asChild>
                  <Link href="/app/dashboard">控制台</Link>
                </BreadcrumbLink>
              </BreadcrumbItem>
              {isSettings && (
                <>
                  <BreadcrumbSeparator className="hidden md:block" />
                  <BreadcrumbItem className="hidden md:block">
                    <BreadcrumbLink asChild>
                      <Link href="/app/settings/profile">设置</Link>
                    </BreadcrumbLink>
                  </BreadcrumbItem>
                </>
              )}
              <BreadcrumbSeparator className="hidden md:block" />
              <BreadcrumbItem>
                <BreadcrumbPage>{pageTitle}</BreadcrumbPage>
              </BreadcrumbItem>
            </BreadcrumbList>
          </Breadcrumb>
        </header>

        {/* 主内容区 */}
        <main className="flex flex-1 flex-col gap-4 p-4 sm:gap-6 sm:p-6" data-testid="app-main">
          {children}
        </main>
      </SidebarInset>
    </SidebarProvider>
  )
}

export default function AppLayout({ children }: { children: React.ReactNode }) {
  return <AppShell>{children}</AppShell>
}
