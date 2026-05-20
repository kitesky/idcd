/**
 * Sidebar navigation data.
 *
 * `title` and group labels hold i18n **keys** (relative to the `userMenu`
 * namespace) — they are resolved at render time by NavGroup via
 * `useTranslations('userMenu')`. This keeps the navigation static & SSR-safe
 * while still being fully translatable.
 */
import {
  Activity,
  BarChart3,
  Bell,
  CreditCard,
  FileText,
  FileWarning,
  Gift,
  Globe,
  LayoutDashboard,
  Lock,
  Server,
  Settings,
  ShieldCheck,
  UserCheck,
} from "lucide-react"
import type { NavGroup } from "./types"

export const NAV_GROUPS: NavGroup[] = [
  {
    title: "sidebar.groups.overview",
    items: [
      { title: "sidebar.items.dashboard", url: "/app/dashboard", icon: LayoutDashboard },
    ],
  },
  {
    title: "sidebar.groups.monitoring",
    items: [
      { title: "sidebar.items.monitors", url: "/app/monitors", icon: Activity },
      {
        title: "sidebar.items.alerts",
        url: undefined,
        icon: Bell,
        items: [
          { title: "sidebar.items.alertsList", url: "/app/alerts" },
          { title: "sidebar.items.alertsChannels", url: "/app/alerts/channels" },
          { title: "sidebar.items.alertsPolicies", url: "/app/alerts/policies" },
          { title: "sidebar.items.alertsGroups", url: "/app/alerts/groups" },
        ],
      },
      { title: "sidebar.items.oncall", url: "/app/oncall", icon: UserCheck },
      { title: "sidebar.items.incidents", url: "/app/incidents", icon: FileWarning },
      { title: "sidebar.items.nodes", url: "/app/nodes", icon: Server },
    ],
  },
  {
    title: "sidebar.groups.publish",
    items: [
      { title: "sidebar.items.statusPages", url: "/app/status-pages", icon: Globe },
    ],
  },
  {
    title: "sidebar.groups.cert",
    items: [
      { title: "sidebar.items.certOverview", url: "/app/cert", icon: Lock },
      { title: "sidebar.items.certNew", url: "/app/cert/new", icon: undefined },
      { title: "sidebar.items.certOrders", url: "/app/cert/orders", icon: undefined },
      { title: "sidebar.items.certCerts", url: "/app/cert/certs", icon: undefined },
      { title: "sidebar.items.certDnsCredentials", url: "/app/cert/dns-credentials", icon: undefined },
    ],
  },
  {
    title: "sidebar.groups.reports",
    items: [
      { title: "sidebar.items.reports", url: "/app/reports", icon: FileText },
      { title: "sidebar.items.verdictNew", url: "/app/verdict/new", icon: ShieldCheck },
    ],
  },
  {
    title: "sidebar.groups.account",
    items: [
      { title: "sidebar.items.billing", url: "/app/billing", icon: CreditCard },
      { title: "sidebar.items.usage", url: "/app/usage", icon: BarChart3 },
      { title: "sidebar.items.referral", url: "/app/referral", icon: Gift },
      {
        title: "sidebar.items.settings",
        url: undefined,
        icon: Settings,
        items: [
          { title: "sidebar.items.settingsProfile", url: "/app/settings/profile" },
          { title: "sidebar.items.settingsAccount", url: "/app/settings/account" },
          { title: "sidebar.items.settingsSecurity", url: "/app/settings/security" },
          { title: "sidebar.items.settingsSessions", url: "/app/settings/sessions" },
          { title: "sidebar.items.settingsApiKeys", url: "/app/settings/api-keys" },
          { title: "sidebar.items.settingsTokens", url: "/app/settings/tokens" },
          { title: "sidebar.items.settingsTeam", url: "/app/settings/team" },
        ],
      },
    ],
  },
]
