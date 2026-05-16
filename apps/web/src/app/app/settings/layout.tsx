import Link from "next/link"
import { ReactNode } from "react"
import { getT } from "@/i18n/getT"

export default async function SettingsLayout({ children }: { children: ReactNode }) {
  const t = await getT("settings")

  const NAV_ITEMS = [
    { href: "/app/settings/profile", label: t("nav.profile") },
    { href: "/app/settings/account", label: t("nav.account") },
    { href: "/app/settings/security", label: t("nav.security") },
    { href: "/app/settings/api-keys", label: t("nav.apiKeys") },
    { href: "/app/settings/tokens", label: t("nav.tokens") },
    { href: "/app/settings/sessions", label: t("nav.sessions") },
    { href: "/app/settings/team", label: t("nav.team") },
  ]

  return (
    <div className="flex flex-col" data-testid="settings-layout">
      <div className="flex-1 flex flex-col gap-8 lg:flex-row">
        {/* ── Sidebar nav ──────────────────────────────────────────── */}
        <nav
          className="flex flex-row flex-wrap gap-1 lg:flex-col lg:flex-nowrap lg:w-48 shrink-0"
          data-testid="settings-nav"
          aria-label={t("title")}
        >
          {NAV_ITEMS.map((item) => (
            <Link
              key={item.href}
              href={item.href}
              className="rounded-md px-3 py-2 text-sm font-medium text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground"
              data-testid={`settings-nav-${item.href.split("/").pop()}`}
            >
              {item.label}
            </Link>
          ))}
        </nav>

        {/* ── Page content ─────────────────────────────────────────── */}
        <div className="flex-1 min-w-0">{children}</div>
      </div>
    </div>
  )
}
