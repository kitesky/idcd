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
  Server,
  Settings,
  UserCheck,
} from "lucide-react"
import type { NavGroup } from "./types"

export const NAV_GROUPS: NavGroup[] = [
  {
    title: "总览",
    items: [
      { title: "仪表盘", url: "/app/dashboard", icon: LayoutDashboard },
    ],
  },
  {
    title: "监控",
    items: [
      { title: "监控列表", url: "/app/monitors", icon: Activity },
      {
        title: "告警管理",
        url: undefined,
        icon: Bell,
        items: [
          { title: "告警列表", url: "/app/alerts" },
          { title: "告警通道", url: "/app/alerts/channels" },
          { title: "告警策略", url: "/app/alerts/policies" },
        ],
      },
      { title: "On-Call 值班", url: "/app/oncall", icon: UserCheck },
      { title: "故障记录", url: "/app/incidents", icon: FileWarning },
      { title: "节点管理", url: "/app/nodes", icon: Server },
    ],
  },
  {
    title: "发布",
    items: [
      { title: "状态页", url: "/app/status-pages", icon: Globe },
    ],
  },
  {
    title: "报告",
    items: [
      { title: "月度报告", url: "/app/reports", icon: FileText },
    ],
  },
  {
    title: "账号",
    items: [
      { title: "订阅与计费", url: "/app/billing", icon: CreditCard },
      { title: "用量统计", url: "/app/usage", icon: BarChart3 },
      { title: "推荐计划", url: "/app/referral", icon: Gift },
      {
        title: "设置",
        url: undefined,
        icon: Settings,
        items: [
          { title: "个人资料", url: "/app/settings/profile" },
          { title: "账户安全", url: "/app/settings/account" },
          { title: "安全设置", url: "/app/settings/security" },
          { title: "会话管理", url: "/app/settings/sessions" },
          { title: "API 密钥", url: "/app/settings/api-keys" },
          { title: "访问令牌", url: "/app/settings/tokens" },
          { title: "团队管理", url: "/app/settings/team" },
        ],
      },
    ],
  },
]
