"use client"

import Link from "next/link"
import { usePathname } from "next/navigation"
import { useTranslations } from "next-intl"
import { ChevronRight } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible"
import {
  SidebarGroup,
  SidebarGroupLabel,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarMenuSub,
  SidebarMenuSubButton,
  SidebarMenuSubItem,
  useSidebar,
} from "@/components/ui/sidebar"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import type { NavCollapsible, NavGroup, NavItem, NavLink } from "./types"

// All sidebar labels live under the `userMenu` namespace — see
// sidebar-data.ts; the `title` field on each entry holds an i18n key
// (e.g. `sidebar.items.dashboard`).
type T = ReturnType<typeof useTranslations<"userMenu">>

export function NavGroup({ title, items }: NavGroup) {
  const { state, isMobile } = useSidebar()
  const pathname = usePathname()
  const t = useTranslations("userMenu")

  return (
    <SidebarGroup>
      <SidebarGroupLabel>{t(title as never)}</SidebarGroupLabel>
      <SidebarMenu>
        {items.map((item) => {
          if (!item.items) {
            return <NavMenuLink key={item.title} item={item as NavLink} pathname={pathname} t={t} />
          }
          if (state === "collapsed" && !isMobile) {
            return <NavMenuCollapsedDropdown key={item.title} item={item as NavCollapsible} pathname={pathname} t={t} />
          }
          return <NavMenuCollapsible key={item.title} item={item as NavCollapsible} pathname={pathname} t={t} />
        })}
      </SidebarMenu>
    </SidebarGroup>
  )
}

function NavMenuLink({ item, pathname, t }: { item: NavLink; pathname: string; t: T }) {
  const { setOpenMobile } = useSidebar()
  const isActive = pathname === item.url || pathname.startsWith(item.url + "/")
  const label = t(item.title as never)
  return (
    <SidebarMenuItem>
      <SidebarMenuButton asChild isActive={isActive} tooltip={label}>
        <Link href={item.url as never} onClick={() => setOpenMobile(false)}>
          {item.icon && <item.icon />}
          <span>{label}</span>
          {item.badge && <Badge className="ml-auto rounded-full px-1 py-0 text-xs">{item.badge}</Badge>}
        </Link>
      </SidebarMenuButton>
    </SidebarMenuItem>
  )
}

function NavMenuCollapsible({ item, pathname, t }: { item: NavCollapsible; pathname: string; t: T }) {
  const { setOpenMobile } = useSidebar()
  const isChildActive = item.items.some((sub) => pathname === sub.url || pathname.startsWith(sub.url + "/"))
  const label = t(item.title as never)

  return (
    <Collapsible asChild defaultOpen={isChildActive} className="group/collapsible">
      <SidebarMenuItem>
        <CollapsibleTrigger asChild>
          <SidebarMenuButton tooltip={label}>
            {item.icon && <item.icon />}
            <span>{label}</span>
            {item.badge && <Badge className="ml-auto rounded-full px-1 py-0 text-xs">{item.badge}</Badge>}
            <ChevronRight className="ml-auto transition-transform duration-200 group-data-[state=open]/collapsible:rotate-90" />
          </SidebarMenuButton>
        </CollapsibleTrigger>
        <CollapsibleContent className="CollapsibleContent">
          <SidebarMenuSub>
            {item.items.map((sub) => {
              const isActive = pathname === sub.url || pathname.startsWith(sub.url + "/")
              return (
                <SidebarMenuSubItem key={sub.title}>
                  <SidebarMenuSubButton asChild isActive={isActive}>
                    <Link href={sub.url as never} onClick={() => setOpenMobile(false)}>
                      {sub.icon && <sub.icon />}
                      <span>{t(sub.title as never)}</span>
                      {sub.badge && <Badge className="ml-auto rounded-full px-1 py-0 text-xs">{sub.badge}</Badge>}
                    </Link>
                  </SidebarMenuSubButton>
                </SidebarMenuSubItem>
              )
            })}
          </SidebarMenuSub>
        </CollapsibleContent>
      </SidebarMenuItem>
    </Collapsible>
  )
}

function NavMenuCollapsedDropdown({ item, pathname, t }: { item: NavCollapsible; pathname: string; t: T }) {
  const isActive = item.items.some((sub) => pathname === sub.url || pathname.startsWith(sub.url + "/"))
  const label = t(item.title as never)
  return (
    <SidebarMenuItem>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <SidebarMenuButton tooltip={label} isActive={isActive}>
            {item.icon && <item.icon />}
            <span>{label}</span>
            <ChevronRight className="ml-auto" />
          </SidebarMenuButton>
        </DropdownMenuTrigger>
        <DropdownMenuContent side="right" align="start" sideOffset={4}>
          <DropdownMenuLabel>{label}</DropdownMenuLabel>
          <DropdownMenuSeparator />
          {item.items.map((sub) => (
            <DropdownMenuItem key={sub.title} asChild>
              <Link href={sub.url as never}>
                {sub.icon && <sub.icon />}
                <span>{t(sub.title as never)}</span>
              </Link>
            </DropdownMenuItem>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>
    </SidebarMenuItem>
  )
}

// helper re-export for external use
export type { NavItem }
