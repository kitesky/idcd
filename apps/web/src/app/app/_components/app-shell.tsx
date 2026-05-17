"use client"

import Link from "next/link"
import { usePathname } from "next/navigation"
import { SidebarProvider, SidebarInset, SidebarTrigger } from "@/components/ui/sidebar"
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

const ROUTE_TITLES: Record<string, string> = {
  "/app/dashboard":          "仪表盘",
  "/app/monitors":           "监控列表",
  "/app/monitors/new":       "创建监控",
  "/app/alerts":             "告警列表",
  "/app/alerts/channels":    "告警通道",
  "/app/alerts/policies":    "告警策略",
  "/app/alerts/groups":      "告警分组",
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

interface AppShellProps {
  email: string
  displayName: string | null
  avatarUrl: string | null
  children: React.ReactNode
}

export function AppShell({ email, displayName, avatarUrl, children }: AppShellProps) {
  const pathname = usePathname()
  const plan = "Free"

  const pageTitle =
    ROUTE_TITLES[pathname] ??
    Object.entries(ROUTE_TITLES).find(([k]) => pathname.startsWith(k + "/"))?.[1] ??
    "后台管理"
  const isSettings = pathname.startsWith("/app/settings")

  return (
    <SidebarProvider>
      <AppSidebar email={email} plan={plan} displayName={displayName} avatarUrl={avatarUrl} />
      <SidebarInset>
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
        <main className="flex flex-1 flex-col gap-4 p-4 sm:gap-6 sm:p-6" data-testid="app-main">
          {children}
        </main>
      </SidebarInset>
    </SidebarProvider>
  )
}
