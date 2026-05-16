"use client"

import Link from "next/link"
import { usePathname } from "next/navigation"
import { useTranslations } from "next-intl"
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

// ── Route → breadcrumb i18n key (relative to `userMenu.sidebar.items`) ──
// Adding a new top-level page: register the key here + add the translation
// under `userMenu.sidebar.items.*` in `messages/{locale}/userMenu.json`.
const ROUTE_TITLE_KEYS: Record<string, string> = {
  "/app/dashboard":          "dashboard",
  "/app/monitors":           "monitors",
  "/app/monitors/new":       "monitorsNew",
  "/app/alerts":             "alertsList",
  "/app/alerts/channels":    "alertsChannels",
  "/app/alerts/policies":    "alertsPolicies",
  "/app/alerts/groups":      "alertsGroups",
  "/app/oncall":             "oncall",
  "/app/incidents":          "incidents",
  "/app/status-pages":       "statusPages",
  "/app/reports":            "reports",
  "/app/billing":            "billing",
  "/app/usage":              "usage",
  "/app/referral":           "referral",
  "/app/settings/profile":   "settingsProfile",
  "/app/settings/account":   "settingsAccount",
  "/app/settings/security":  "settingsSecurity",
  "/app/settings/sessions":  "settingsSessions",
  "/app/settings/api-keys":  "settingsApiKeys",
  "/app/settings/tokens":    "settingsTokens",
  "/app/settings/team":      "settingsTeam",
  "/app/nodes":              "nodes",
}

interface AppShellProps {
  email: string
  displayName: string | null
  avatarUrl: string | null
  plan?: string
  children: React.ReactNode
}

export function AppShell({ email, displayName, avatarUrl, plan = "Free", children }: AppShellProps) {
  const pathname = usePathname()
  const tSidebar = useTranslations("userMenu.sidebar")
  const tItems = useTranslations("userMenu.sidebar.items")

  const titleKey =
    ROUTE_TITLE_KEYS[pathname] ??
    Object.entries(ROUTE_TITLE_KEYS).find(([k]) => pathname.startsWith(k + "/"))?.[1] ??
    null
  const pageTitle = titleKey ? tItems(titleKey) : tSidebar("defaultPageTitle")
  const isSettings = pathname.startsWith("/app/settings")

  return (
    <SidebarProvider>
      <AppSidebar email={email} plan={plan} displayName={displayName} avatarUrl={avatarUrl} />
      <SidebarInset>
        <header className="flex h-12 shrink-0 items-center gap-2 border-b px-4">
          <SidebarTrigger className="-ml-1" data-testid="mobile-menu-button" aria-label={tSidebar("openMenu")} />
          <Separator orientation="vertical" className="mr-2 h-4" />
          <Breadcrumb>
            <BreadcrumbList>
              <BreadcrumbItem className="hidden md:block">
                <BreadcrumbLink asChild>
                  <Link href="/app/dashboard">{tSidebar("console")}</Link>
                </BreadcrumbLink>
              </BreadcrumbItem>
              {isSettings && (
                <>
                  <BreadcrumbSeparator className="hidden md:block" />
                  <BreadcrumbItem className="hidden md:block">
                    <BreadcrumbLink asChild>
                      <Link href="/app/settings/profile">{tSidebar("settingsCrumb")}</Link>
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
