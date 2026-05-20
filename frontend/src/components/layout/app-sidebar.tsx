"use client"

import Link from "next/link"
import { useTranslations } from "next-intl"
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarRail,
} from "@/components/ui/sidebar"
import { NavGroup } from "./nav-group"
import { NavUser } from "./nav-user"
import { NAV_GROUPS } from "./sidebar-data"

type AppSidebarProps = {
  email?: string
  plan?: string
  displayName?: string | null
  avatarUrl?: string | null
} & React.ComponentProps<typeof Sidebar>

export function AppSidebar({ email = "user@example.com", plan = "Free", displayName, avatarUrl, ...props }: AppSidebarProps) {
  const t = useTranslations("userMenu.sidebar")
  return (
    <Sidebar collapsible="icon" data-testid="desktop-sidebar" {...props}>
      <SidebarHeader>
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton size="lg" asChild>
              <Link href="/" data-testid="logo-link">
                <div className="flex aspect-square size-8 items-center justify-center rounded-lg bg-primary text-primary-foreground font-mono font-bold text-sm">
                  id
                </div>
                <div className="flex flex-col gap-0.5 leading-none">
                  <span className="font-semibold font-mono">idcd</span>
                  <span className="text-xs text-muted-foreground">{t("brandTagline")}</span>
                </div>
              </Link>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarHeader>

      <SidebarContent>
        {NAV_GROUPS.map((group) => (
          <NavGroup key={group.title} {...group} />
        ))}
      </SidebarContent>

      <SidebarFooter>
        <NavUser email={email} plan={plan} displayName={displayName} avatarUrl={avatarUrl} />
      </SidebarFooter>

      <SidebarRail />
    </Sidebar>
  )
}
