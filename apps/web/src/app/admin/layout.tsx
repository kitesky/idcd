import Link from "next/link"
import { cookies } from "next/headers"
import { getT } from "@/i18n/getT"
import { isValidLocale, defaultLocale, type Locale } from "@/i18n/routing"
import { LanguageSwitcher } from "./lang-switcher"

async function getAdminLocale(): Promise<Locale> {
  const cookieStore = await cookies()
  const val = cookieStore.get("locale")?.value ?? ""
  return isValidLocale(val) ? val : defaultLocale
}

export default async function AdminLayout({ children }: { children: React.ReactNode }) {
  const locale = await getAdminLocale()
  const t = await getT("admin", locale)

  const NAV = [
    { href: "/admin/metrics",          label: t("nav.metrics") },
    { href: "/admin/users",            label: t("nav.users") },
    { href: "/admin/nodes",            label: t("nav.nodes") },
    { href: "/admin/node-applications", label: t("nav.nodeApplications") },
    { href: "/admin/refund-failed",    label: t("nav.refundFailed") },
    { href: "/admin/beta-invitations", label: t("nav.betaInvitations") },
    { href: "/admin/upgrades",         label: t("nav.upgrades") },
    { href: "/admin/cert",             label: t("nav.cert") },
  ]

  return (
    <div className="flex min-h-screen flex-col">
      <header className="border-b bg-card px-6 py-3">
        <div className="flex items-center justify-between gap-6">
          <div className="flex items-center gap-6">
            <Link href={"/admin" as any} className="text-base font-semibold text-primary">
              idcd Admin
            </Link>
            <nav className="flex gap-4 text-sm">
              {NAV.map(item => (
                <Link
                  key={item.href}
                  href={item.href as any}
                  className="text-muted-foreground transition-colors hover:text-foreground"
                >
                  {item.label}
                </Link>
              ))}
            </nav>
          </div>
          <LanguageSwitcher currentLocale={locale} label={t("lang.switchTo")} />
        </div>
      </header>
      <main className="flex-1 container mx-auto px-6 py-6">{children}</main>
    </div>
  )
}
