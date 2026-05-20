import { ReactNode } from "react"
import { getT } from "@/i18n/getT"
import { SettingsNav } from "./settings-nav"

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
        <SettingsNav items={NAV_ITEMS} ariaLabel={t("title")} />

        {/* ── Page content ─────────────────────────────────────────── */}
        <div className="flex-1 min-w-0">{children}</div>
      </div>
    </div>
  )
}
