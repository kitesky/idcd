"use client"

import Link from "next/link"
import { useTranslations } from "next-intl"
import { BadgeCheck, Bell, ChevronsUpDown, CreditCard, LogOut, Settings } from "lucide-react"
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  useSidebar,
} from "@/components/ui/sidebar"
import { Badge } from "@/components/ui/badge"

type NavUserProps = {
  email: string
  plan?: string
  displayName?: string | null
  avatarUrl?: string | null
}

export function NavUser({ email, plan = "Free", displayName, avatarUrl }: NavUserProps) {
  const { isMobile } = useSidebar()
  const t = useTranslations("userMenu")

  const initial = (displayName ?? email).charAt(0).toUpperCase()
  const planVariant =
    plan === "Pro" ? "default" :
    plan === "Team" || plan === "Business" ? "secondary" :
    "outline"

  return (
    <SidebarMenu>
      <SidebarMenuItem>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <SidebarMenuButton
              size="lg"
              className="data-[state=open]:bg-sidebar-accent data-[state=open]:text-sidebar-accent-foreground"
              data-testid="user-menu-trigger"
            >
              <Avatar className="h-8 w-8 rounded-lg">
                <AvatarImage src={avatarUrl ?? undefined} alt={displayName ?? email} />
                <AvatarFallback className="rounded-lg bg-primary/10 text-primary text-sm font-semibold">
                  {initial}
                </AvatarFallback>
              </Avatar>
              <div className="grid flex-1 text-left text-sm leading-tight">
                <span className="truncate font-semibold">{displayName ?? email}</span>
                {displayName && (
                  <span className="truncate text-xs text-muted-foreground">{email}</span>
                )}
                {!displayName && (
                  <span className="truncate text-xs text-muted-foreground" data-testid="plan-badge">
                    {t("plan.label", { plan })}
                  </span>
                )}
              </div>
              <ChevronsUpDown className="ml-auto size-4" />
            </SidebarMenuButton>
          </DropdownMenuTrigger>
          <DropdownMenuContent
            className="w-(--radix-dropdown-menu-trigger-width) min-w-56 rounded-lg"
            side={isMobile ? "bottom" : "right"}
            align="end"
            sideOffset={4}
          >
            <DropdownMenuLabel className="p-0 font-normal">
              <div className="flex items-center gap-2 px-1 py-1.5">
                <Avatar className="h-8 w-8 rounded-lg">
                  <AvatarImage src={avatarUrl ?? undefined} alt={displayName ?? email} />
                  <AvatarFallback className="rounded-lg bg-primary/10 text-primary text-sm font-semibold">
                    {initial}
                  </AvatarFallback>
                </Avatar>
                <div className="grid flex-1 text-left text-sm leading-tight">
                  <span className="truncate font-semibold">{displayName ?? email}</span>
                  {displayName && (
                    <span className="truncate text-xs text-muted-foreground">{email}</span>
                  )}
                  <Badge variant={planVariant as "default" | "secondary" | "outline"} className="mt-0.5 w-fit text-xs">
                    {plan}
                  </Badge>
                </div>
              </div>
            </DropdownMenuLabel>
            <DropdownMenuSeparator />
            <DropdownMenuGroup>
              <DropdownMenuItem asChild>
                <Link href="/app/billing">
                  <CreditCard />
                  {t("items.billing")}
                </Link>
              </DropdownMenuItem>
              <DropdownMenuItem asChild>
                <Link href="/app/settings/profile">
                  <BadgeCheck />
                  {t("items.profile")}
                </Link>
              </DropdownMenuItem>
              <DropdownMenuItem asChild>
                <Link href="/app/settings/account">
                  <Settings />
                  {t("items.account")}
                </Link>
              </DropdownMenuItem>
              <DropdownMenuItem asChild>
                <Link href="/app/alerts/channels">
                  <Bell />
                  {t("items.alertChannels")}
                </Link>
              </DropdownMenuItem>
            </DropdownMenuGroup>
            <DropdownMenuSeparator />
            <DropdownMenuItem asChild className="text-destructive focus:text-destructive">
              <Link href="/auth/logout">
                <LogOut />
                {t("items.logout")}
              </Link>
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </SidebarMenuItem>
    </SidebarMenu>
  )
}
