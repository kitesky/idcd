import Link from "next/link"
import { ReactNode } from "react"

const NAV_ITEMS = [
  { href: "/app/settings/profile", label: "个人资料" },
  { href: "/app/settings/account", label: "账号安全" },
  { href: "/app/settings/api-keys", label: "API Keys" },
  { href: "/app/settings/tokens", label: "访问令牌" },
] as const

export default function SettingsLayout({ children }: { children: ReactNode }) {
  return (
    <div className="flex min-h-screen flex-col" data-testid="settings-layout">
      <div className="container max-w-5xl flex-1 flex flex-col py-8 gap-8 lg:flex-row">
        {/* ── Sidebar nav ──────────────────────────────────────────── */}
        <nav
          className="flex flex-row gap-1 lg:flex-col lg:w-48 shrink-0"
          data-testid="settings-nav"
          aria-label="设置导航"
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
