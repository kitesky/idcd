"use client"

import { useState } from "react"
import { CheckCircle2, AlertTriangle, XCircle, ChevronDown, ChevronRight, ExternalLink } from "lucide-react"
import { cn } from "@/lib/utils"
import type { StatusPageData, ServiceStatus } from "./mock-data"
import { generateUptimeHistory } from "./mock-data"

// ── Minimal shadcn-style components (no shadcn package in status app) ──────────

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
        "rounded-lg border border-[hsl(217.2_32.6%_17.5%)] bg-[hsl(222.2_84%_4.9%)] text-[hsl(210_40%_98%)]",
        className
      )}
    >
      {children}
    </div>
  )
}

// ── Status helpers ─────────────────────────────────────────────────────────────

function overallStatusConfig(status: ServiceStatus) {
  switch (status) {
    case "operational":
      return {
        label: "全部服务正常",
        variant: "success" as const,
        icon: <CheckCircle2 className="h-6 w-6 text-green-400" />,
        bgClass: "bg-green-900/20 border-green-800",
      }
    case "degraded":
      return {
        label: "部分服务降级",
        variant: "warning" as const,
        icon: <AlertTriangle className="h-6 w-6 text-yellow-400" />,
        bgClass: "bg-yellow-900/20 border-yellow-800",
      }
    case "outage":
      return {
        label: "严重服务中断",
        variant: "destructive" as const,
        icon: <XCircle className="h-6 w-6 text-red-400" />,
        bgClass: "bg-red-900/20 border-red-800",
      }
    case "maintenance":
      return {
        label: "计划维护中",
        variant: "secondary" as const,
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
      aria-label={status}
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

// ── Main client component ──────────────────────────────────────────────────────

interface StatusClientProps {
  data: StatusPageData
}

export function StatusClient({ data }: StatusClientProps) {
  const [expandedGroups, setExpandedGroups] = useState<Set<string>>(
    new Set(data.groups.map((g) => g.id))
  )

  const statusCfg = overallStatusConfig(data.overallStatus)
  const uptimeHistory = generateUptimeHistory(99.5)

  function toggleGroup(id: string) {
    setExpandedGroups((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  return (
    <div className="min-h-screen bg-[hsl(222.2_84%_4.9%)] text-[hsl(210_40%_98%)]">
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
                  <div className="border-t border-[hsl(217.2_32.6%_17.5%)] divide-y divide-[hsl(217.2_32.6%_17.5%)]">
                    {group.monitors.map((monitor) => (
                      <div
                        key={monitor.id}
                        className="flex items-center justify-between px-5 py-3"
                        data-testid={`monitor-row-${monitor.id}`}
                      >
                        <span className="text-sm">{monitor.name}</span>
                        <div className="flex items-center gap-2">
                          <span className="text-xs text-[hsl(215_20.2%_65.1%)]">
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
                  className={cn(
                    "h-5 w-full rounded-sm",
                    uptimeDayColor(day.status)
                  )}
                />
              ))}
            </div>
            <div className="mt-3 flex items-center justify-between text-xs text-[hsl(215_20.2%_65.1%)]">
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
            <Card className="px-5 py-8 text-center text-[hsl(215_20.2%_65.1%)] text-sm">
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
                  <p className="text-xs text-[hsl(215_20.2%_65.1%)] leading-relaxed mb-3">
                    {evt.description}
                  </p>
                  <div className="text-xs text-[hsl(215_20.2%_65.1%)] space-y-1">
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

        {/* ── Footer Branding ── */}
        {data.showBranding && (
          <footer className="text-center" data-testid="powered-by">
            <a
              href="https://idcd.com"
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-1.5 text-xs text-[hsl(215_20.2%_65.1%)] hover:text-[hsl(210_40%_98%)] transition-colors"
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
