import type { Metadata } from "next"
import Link from "next/link"
import {
  Activity,
  AlertTriangle,
  ArrowRight,
  Bell,
  CheckCircle2,
  Globe,
  LayoutDashboard,
  Plus,
  TrendingUp,
} from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"

export const metadata: Metadata = {
  title: "总览 - idcd",
  description: "idcd 控制台总览：监控状态、可用率、告警汇总",
}

// ─── Mock data ────────────────────────────────────────────────────────────────

const SUMMARY = {
  monitors: { total: 5, up: 4, down: 1, paused: 0 },
  checksToday: 1440,
  avgUptime7d: 99.7,
  incidentsOpen: 1,
  alertsFired7d: 3,
  statusPages: 2,
}

type AlertStatus = "firing" | "resolved"

interface RecentAlert {
  id: string
  time: string
  monitorName: string
  status: AlertStatus
  channel: string
}

const RECENT_ALERTS: RecentAlert[] = [
  { id: "ae-001", time: "2026-05-14 09:12", monitorName: "API 网关健康检查", status: "firing", channel: "邮件" },
  { id: "ae-002", time: "2026-05-14 07:45", monitorName: "idcd.com 主站", status: "resolved", channel: "Webhook" },
  { id: "ae-003", time: "2026-05-13 23:18", monitorName: "香港节点 Ping", status: "resolved", channel: "邮件" },
  { id: "ae-004", time: "2026-05-13 18:02", monitorName: "API 网关健康检查", status: "resolved", channel: "邮件" },
  { id: "ae-005", time: "2026-05-13 11:55", monitorName: "DNS 解析检查", status: "resolved", channel: "Webhook" },
]

// ─── Stat card ────────────────────────────────────────────────────────────────

interface StatCardProps {
  title: string
  value: string | number
  icon: React.ReactNode
  badge?: React.ReactNode
  testId?: string
}

function StatCard({ title, value, icon, badge, testId }: StatCardProps) {
  return (
    <Card data-testid={testId}>
      <CardHeader className="pb-2">
        <CardTitle className="flex items-center gap-2 text-sm font-medium text-muted-foreground">
          {icon}
          {title}
        </CardTitle>
      </CardHeader>
      <CardContent className="flex items-center gap-2">
        <p className="text-3xl font-bold tabular-nums">{value}</p>
        {badge}
      </CardContent>
    </Card>
  )
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export default function DashboardPage() {
  const { monitors, checksToday, avgUptime7d, incidentsOpen, alertsFired7d, statusPages } = SUMMARY

  return (
    <div className="min-h-screen bg-background">
      <div className="container mx-auto px-4 py-8">
        <div className="mb-8">
          <h1 className="text-3xl font-bold tracking-tight">总览</h1>
          <p className="mt-2 text-muted-foreground">
            监控健康状态与关键指标一览
          </p>
        </div>

        <div className="space-y-8">
          {/* ── 第一行：6 个统计卡片 ─────────────────────────────────────────── */}
          <div
            className="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-6"
            data-testid="stat-cards"
          >
            <StatCard
              testId="stat-monitors-total"
              title="监控总数"
              value={monitors.total}
              icon={<Activity className="h-4 w-4" />}
            />

            <StatCard
              testId="stat-monitors-up"
              title="在线 / 总数"
              value={`${monitors.up} / ${monitors.total}`}
              icon={<CheckCircle2 className="h-4 w-4 text-success" />}
              badge={<Badge variant="success">正常</Badge>}
            />

            <StatCard
              testId="stat-checks-today"
              title="今日拨测次数"
              value={checksToday.toLocaleString()}
              icon={<TrendingUp className="h-4 w-4" />}
            />

            <StatCard
              testId="stat-uptime-7d"
              title="7 日平均可用率"
              value={`${avgUptime7d}%`}
              icon={<Activity className="h-4 w-4 text-success" />}
            />

            <StatCard
              testId="stat-incidents-open"
              title="未解决告警"
              value={incidentsOpen}
              icon={<AlertTriangle className="h-4 w-4 text-destructive" />}
              badge={
                incidentsOpen > 0 ? (
                  <Badge variant="destructive">{incidentsOpen}</Badge>
                ) : undefined
              }
            />

            <StatCard
              testId="stat-status-pages"
              title="状态页数量"
              value={statusPages}
              icon={<Globe className="h-4 w-4" />}
            />
          </div>

          {/* ── 第二行：快捷入口 ─────────────────────────────────────────────── */}
          <div>
            <h2 className="mb-4 text-lg font-semibold">快捷入口</h2>
            <div
              className="grid grid-cols-1 gap-4 sm:grid-cols-3"
              data-testid="quick-links"
            >
              <Card className="transition-colors hover:bg-muted/50">
                <CardHeader>
                  <CardTitle className="flex items-center gap-2 text-base">
                    <Plus className="h-5 w-5 text-primary" />
                    新建监控
                  </CardTitle>
                  <CardDescription>添加 HTTP、Ping、SSL 或 DNS 监控项目</CardDescription>
                </CardHeader>
                <CardContent>
                  <Button asChild variant="outline" size="sm">
                    <Link href="/app/monitors/new" data-testid="link-new-monitor">
                      立即创建
                      <ArrowRight className="ml-2 h-4 w-4" />
                    </Link>
                  </Button>
                </CardContent>
              </Card>

              <Card className="transition-colors hover:bg-muted/50">
                <CardHeader>
                  <CardTitle className="flex items-center gap-2 text-base">
                    <Bell className="h-5 w-5 text-primary" />
                    查看告警
                  </CardTitle>
                  <CardDescription>管理告警通道、策略和历史事件</CardDescription>
                </CardHeader>
                <CardContent>
                  <Button asChild variant="outline" size="sm">
                    <Link href="/app/alerts" data-testid="link-alerts">
                      前往告警
                      <ArrowRight className="ml-2 h-4 w-4" />
                    </Link>
                  </Button>
                </CardContent>
              </Card>

              <Card className="transition-colors hover:bg-muted/50">
                <CardHeader>
                  <CardTitle className="flex items-center gap-2 text-base">
                    <LayoutDashboard className="h-5 w-5 text-primary" />
                    管理状态页
                  </CardTitle>
                  <CardDescription>发布和配置对外公开的服务状态页面</CardDescription>
                </CardHeader>
                <CardContent>
                  <Button asChild variant="outline" size="sm">
                    <Link href="/app/status-pages" data-testid="link-status-pages">
                      管理状态页
                      <ArrowRight className="ml-2 h-4 w-4" />
                    </Link>
                  </Button>
                </CardContent>
              </Card>
            </div>
          </div>

          {/* ── 第三行：近期告警事件 ─────────────────────────────────────────── */}
          <div>
            <div className="mb-4 flex items-center justify-between">
              <h2 className="text-lg font-semibold">近期告警事件</h2>
              <Button asChild variant="ghost" size="sm">
                <Link href="/app/alerts">
                  查看全部
                  <ArrowRight className="ml-2 h-4 w-4" />
                </Link>
              </Button>
            </div>
            <Card data-testid="recent-alerts-table">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>时间</TableHead>
                    <TableHead>监控名</TableHead>
                    <TableHead>状态</TableHead>
                    <TableHead>通道</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {RECENT_ALERTS.map((alert) => (
                    <TableRow key={alert.id}>
                      <TableCell className="font-mono text-xs text-muted-foreground">
                        {alert.time}
                      </TableCell>
                      <TableCell className="font-medium">{alert.monitorName}</TableCell>
                      <TableCell>
                        {alert.status === "firing" ? (
                          <Badge variant="destructive">告警中</Badge>
                        ) : (
                          <Badge variant="success">已恢复</Badge>
                        )}
                      </TableCell>
                      <TableCell className="text-sm text-muted-foreground">
                        {alert.channel}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </Card>
          </div>
        </div>
      </div>
    </div>
  )
}
