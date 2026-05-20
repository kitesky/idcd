"use client"

import Link from "next/link"
import { usePathname } from "next/navigation"
import { cn } from "@/lib/utils"

interface NavItem {
  href: string
  label: string
}

interface Props {
  items: NavItem[]
  ariaLabel: string
}

// SettingsNav renders the settings sidebar with the current route highlighted.
// Kept as a tiny client component so the parent layout can stay a Server
// Component (it needs getT for SSR translation) — pathname is the only piece
// of state the nav needs and there's no value in shipping any other logic to
// the browser.
export function SettingsNav({ items, ariaLabel }: Props) {
  const pathname = usePathname()

  return (
    <nav
      className="flex flex-row flex-wrap gap-1 lg:flex-col lg:flex-nowrap lg:w-48 shrink-0"
      data-testid="settings-nav"
      aria-label={ariaLabel}
    >
      {items.map((item) => {
        // Use startsWith so /app/settings/team/* still highlights "Team".
        const isActive =
          pathname === item.href || pathname.startsWith(`${item.href}/`)
        return (
          <Link
            key={item.href}
            href={item.href}
            aria-current={isActive ? "page" : undefined}
            className={cn(
              "rounded-md px-3 py-2 text-sm font-medium transition-colors",
              isActive
                ? "bg-accent text-accent-foreground"
                : "text-muted-foreground hover:bg-accent hover:text-accent-foreground",
            )}
            data-testid={`settings-nav-${item.href.split("/").pop()}`}
          >
            {item.label}
          </Link>
        )
      })}
    </nav>
  )
}
