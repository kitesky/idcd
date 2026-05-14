"use client"

import { useMemo, useState } from "react"
import { CheckCircle2, AlertTriangle, XCircle, ChevronDown, ChevronRight, ExternalLink } from "lucide-react"
import { cn } from "@/lib/utils"
import type { StatusPageData, ServiceStatus } from "./mock-data"
import { generateUptimeHistory } from "./mock-data"

// ── Minimal shadcn-compatible components ──────────────────────────────────────────────
// shadcn/ui is not installed in this standalone app; these thin wrappers mirror
// the shadcn API surface and use the CSS variables defined in globals.css so
// that a later migration to the real shadcn package is a one-line swap.

function Badge({
  variant,
  className,
  children,
}: {
  variant: "success" | "warning" | "destructive" | "secondary" | "outline"
  className?: string
  children: React.ReactNode
}) {
  const base =
    "inline-flex items-center rounded-md border px-2.5 py-0.5 text-xs font-semibold"
  const variants: Record<string, string> = {
    success: "border-transparent bg-green-600/20 text-green-400",
    warning: "border-transparent bg-yellow-600/20 text-yellow-400",
    destructive: "border-transparent bg-red-600/20 text-red-400",
    secondary: "border-transparent bg-secondary text-secondary-foreground",
    outline: "text-foreground border-border",
  }
  return (
    <span className={cn(base, variants[variant], className)}>{children}</span>
  )
}

function Card({
  className,
  children,
}: {
  className?: string
  children: React.ReactNode
}) {
  return (
    <div
      className={cn(
        "rounded-lg border border-border bg-card text-card-foreground",
        className
      )}
    >
      {children}
    </div>
  )
}

// ── Status helpers ─────────────────────────────────────────────────────────────────────────────

/** Maps ServiceStatus to a Chinese label for use in aria attributes. */
const STATUS_LABEL_ZH: Record<ServiceStatus, string> = {
  operational: "正常",
  degraded: "降级",
  outage: "中断",
  maintenance: "维护中",
}

function overallStatusConfig(status: ServiceStatus): {
  label: string
  variant: "success" | "warning" | "destructive" | "secondary"
  icon: React.ReactNode
  bgClass: string
} {
  switch (status) {
    case "operational":
      return {
        label: "全部服务正常",
        variant: "success",
        icon: <CheckCircle2 className="h-6 w-6 text-green-400" />,
        bgClass: "bg-green-900/20 border-green-800",
      }
    case "degraded":
      return {
        label: "部分服务降级",
        variant: "warning",
        icon: <AlertTriangle className="h-6 w-6 text-yellow-400" />,
        bgClass: "bg-yellow-900/20 border-yellow-800",
      }
    case "outage":
      return {
        label: "严重服务中断",
        variant: "destructive",
        icon: <XCircle className="h-6 w-6 text-red-400" />,
        bgClass: "bg-red-900/20 border-red-800",
      }
    case "maintenance":
      return {
        label: "计划维护中",
        variant: "secondary",
        icon: <AlertTriangle className="h-6 w-6 text-blue-400" />,
        bgClass: "bg-blue-900/20 border-blue-800",
      }
  }
}

function monitorStatusDot(status: ServiceStatus) {
  const colors: Record<ServiceStatus, string> = {
    operational: "bg-green-500",
    degraded: "bg-yellow-500",
    outage: "bg-red-500",
    maintenance: "bg-blue-500",
  }
  return (
    <span
      aria-label={STATUS_LABEL_ZH[status]}
      role="img"
      className={cn(
        "inline-block h-2.5 w-2.5 rounded-full",
        colors[status]
      )}
    />
  )
}

function uptimeDayColor(status: ServiceStatus): string {
  const colors: Record<ServiceStatus, string> = {
    operational: "bg-green-600",
    degraded: "bg-yellow-500",
    outage: "bg-red-600",
    maintenance: "bg-blue-600",
  }
  return colors[status]
}

function formatDate(iso: string) {
  return new Date(iso).toLocaleString("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    timeZone: "UTC",
    timeZoneName: "short",
  })
}

// ── Main client component ──────────────────────────────────────────────────────────────────────────────

interface StatusClientProps {
  data: StatusPageData
}

export function StatusClient({ data }: StatusClientProps) {
  const [expandedGroups, setExpandedGroups] = useState<Set<string>>(
    new Set(data.groups.map((g) => g.id))
  )
  const [subscribeEmail, setSubscribeEmail] = useState("")
  const [subscribeStatus, setSubscribeStatus] = useState<"idle" | "loading" | "success" | "error">("idle")
  const [subscribeError, setSubscribeError] = useState("")

  const statusCfg = overallStatusConfig(data.overallStatus)

  // Memoize so that the 90-day grid is stable across re-renders (group
  // expand/collapse toggling must not re-randomize the history blocks).
  const uptimeHistory = useMemo(() => generateUptimeHistory(99.5), [])

  function toggleGroup(id: string) {
    setExpandedGroups((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  async function handleSubscribe(e: React.FormEvent) {
    e.preventDefault()
    if (!subscribeEmail.trim()) return
    setSubscribeStatus("loading")
    setSubscribeError("")
    try {
      const apiBase = process.env.NEXT_PUBLIC_API_URL ?? ""
      const res = await fetch(`${apiBase}/api/v1/status-pages/${data.slug}/subscribe`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ channel_type: "email", endpoint: subscribeEmail.trim(), events: ["incident", "recovery"] }),
      })
      if (!res.ok) {
        const json = await res.json().catch(() => ({}))
        setSubscribeError((json as { error?: { message?: string } })?.error?.message ?? "订阅失败，请重试")
        setSubscribeStatus("error")
        return
      }
      setSubscribeStatus("success")
      setSubscribeEmail("")
    } catch {
      setSubscribeError("网络错误，请重试")
      setSubscribeStatus("error")
    }
  }

  return (
    <div className="min-h-screen bg-background text-foreground">
      <div className="mx-auto max-w-3xl px-4 py-12">

        {/* ── Header ── */}
        <div className="mb-10 text-center">
          <h1 className="text-3xl font-bold tracking-tight mb-4" data-testid="status-title">
            {data.title}
          </h1>
          <div
            className={cn(
              "inline-flex items-center gap-3 rounded-xl border px-6 py-3",
              statusCfg.bgClass
            )}
            data-testid="overall-status"
          >
            {statusCfg.icon}
            <Badge variant={statusCfg.variant} className="text-sm px-3 py-1">
              {statusCfg.label}
            </Badge>
          </div>
        </div>

        {/* ── Service Groups ── */}
        <div className="mb-10 space-y-4" data-testid="service-groups">
          {data.groups.map((group) => {
            const expanded = expandedGroups.has(group.id)
            return (
              <Card key={group.id}>
                <button
                  className="flex w-full items-center justify-between px-5 py-4 text-left hover:bg-white/5 transition-colors"
                  onClick={() => toggleGroup(group.id)}
                  aria-expanded={expanded}
                  data-testid={`group-toggle-${group.id}`}
                >
                  <span className="font-semibold text-base">{group.name}</span>
                  {expanded ? (
                    <ChevronDown className="h-4 w-4 text-muted-foreground" />
                  ) : (
                    <ChevronRight className="h-4 w-4 text-muted-foreground" />
                  )}
                </button>

                {expanded && (
                  <div className="border-t border-border divide-y divide-border">
                    {group.monitors.map((monitor) => (
                      <div
                        key={monitor.id}
                        className="flex items-center justify-between px-5 py-3"
                        data-testid={`monitor-row-${monitor.id}`}
                      >
                        <span className="text-sm">{monitor.name}</span>
                        <div className="flex items-center gap-2">
                          <span className="text-xs text-muted-foreground">
                            {monitor.uptimePercent.toFixed(2)}%
                          </span>
                          {monitorStatusDot(monitor.status)}
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </Card>
            )
          })}
        </div>

        {/* ── 90-Day Uptime History ── */}
        <div className="mb-10" data-testid="uptime-history">
          <h2 className="text-lg font-semibold mb-4">历史可用率（过去 90 天）</h2>
          <Card className="p-5">
            <div
              className="grid gap-0.5"
              style={{ gridTemplateColumns: "repeat(90, 1fr)" }}
              aria-label="90天可用率方块图"
              data-testid="uptime-grid"
            >
              {uptimeHistory.map((day, i) => (
                <div
                  key={i}
                  title={`${day.date}: ${day.uptime.toFixed(1)}%`}
                  aria-label={`${day.date} 可用率 ${day.uptime.toFixed(1)}%，状态：${STATUS_LABEL_ZH[day.status]}`}
                  className={cn(
                    "h-5 w-full rounded-sm",
                    uptimeDayColor(day.status)
                  )}
                />
              ))}
            </div>
            <div className="mt-3 flex items-center justify-between text-xs text-muted-foreground">
              <span>90 天前</span>
              <div className="flex items-center gap-3">
                <span className="flex items-center gap-1">
                  <span className="inline-block h-2.5 w-2.5 rounded-sm bg-green-600" />
                  正常
                </span>
                <span className="flex items-center gap-1">
                  <span className="inline-block h-2.5 w-2.5 rounded-sm bg-yellow-500" />
                  降级
                </span>
                <span className="flex items-center gap-1">
                  <span className="inline-block h-2.5 w-2.5 rounded-sm bg-red-600" />
                  中断
                </span>
              </div>
              <span>今天</span>
            </div>
          </Card>
        </div>

        {/* ── Recent Events ── */}
        <div className="mb-10" data-testid="recent-events">
          <h2 className="text-lg font-semibold mb-4">最近事件公告</h2>
          {data.events.length === 0 ? (
            <Card className="px-5 py-8 text-center text-muted-foreground text-sm">
              最近 90 天内无事件记录
            </Card>
          ) : (
            <div className="space-y-3">
              {data.events.map((evt) => (
                <Card key={evt.id} className="px-5 py-4">
                  <div className="flex items-start justify-between gap-4 mb-2">
                    <h3 className="font-medium text-sm">{evt.title}</h3>
                    <Badge
                      variant={evt.status === "resolved" ? "success" : "warning"}
                    >
                      {evt.status === "resolved" ? "已解决" : "处理中"}
                    </Badge>
                  </div>
                  <p className="text-xs text-muted-foreground leading-relaxed mb-3">
                    {evt.description}
                  </p>
                  <div className="text-xs text-muted-foreground space-y-1">
                    <div>发生时间：{formatDate(evt.createdAt)}</div>
                    {evt.resolvedAt && (
                      <div>解决时间：{formatDate(evt.resolvedAt)}</div>
                    )}
                    <div>影响服务：{evt.affectedServices.join("、")}</div>
                  </div>
                </Card>
              ))}
            </div>
          )}
        </div>

        {/* ── Subscribe to Updates ── */}
        <div className="mb-10" data-testid="subscribe-section">
          <h2 className="text-lg font-semibold mb-4">订阅状态更新</h2>
          <Card className="px-5 py-5">
            <div className="flex flex-wrap gap-2 mb-4">
              <Badge variant="secondary">邮件</Badge>
              <Badge variant="secondary">Webhook</Badge>
              <Badge variant="secondary">企业微信</Badge>
              <Badge variant="secondary">钉钉</Badge>
            </div>
            {subscribeStatus === "success" ? (
              <div className="rounded-md border border-green-600/30 bg-green-600/10 px-4 py-3 text-sm text-green-400" role="alert">
                验证邮件已发送，请查收并点击链接完成订阅。
              </div>
            ) : (
              <form onSubmit={handleSubscribe} className="flex gap-2">
                <input
                  type="email"
                  placeholder="your@email.com"
                  value={subscribeEmail}
                  onChange={(e) => setSubscribeEmail(e.target.value)}
                  required
                  className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm transition-colors placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50"
                />
                <button
                  type="submit"
                  disabled={subscribeStatus === "loading"}
                  className="inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-md text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50 bg-primary text-primary-foreground shadow hover:bg-primary/90 h-9 px-4 py-2"
                >
                  {subscribeStatus === "loading" ? "发送中…" : "订阅"}
                </button>
              </form>
            )}
            {subscribeStatus === "error" && (
              <p className="mt-2 text-xs text-destructive">{subscribeError}</p>
            )}
          </Card>
        </div>

        {/* ── Footer Branding ── */}
        {data.showBranding && (
          <footer className="text-center" data-testid="powered-by">
            <a
              href="https://idcd.com"
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors"
            >
              Powered by idcd
              <ExternalLink className="h-3 w-3" />
            </a>
          </footer>
        )}
      </div>
    </div>
  )
}
