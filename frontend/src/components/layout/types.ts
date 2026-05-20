import type { LucideIcon } from "lucide-react"

export type NavLink = {
  title: string
  url: string
  icon?: LucideIcon
  badge?: string
  items?: never
}

export type NavCollapsible = {
  title: string
  url?: never
  icon?: LucideIcon
  badge?: string
  items: { title: string; url: string; icon?: LucideIcon; badge?: string }[]
}

export type NavItem = NavLink | NavCollapsible

export type NavGroup = {
  title: string
  items: NavItem[]
}
