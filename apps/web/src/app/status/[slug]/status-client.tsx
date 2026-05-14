"use client"

import { useMemo, useState } from "react"
import { CheckCircle2, AlertTriangle, XCircle, ChevronDown, ChevronRight, ExternalLink } from "lucide-react"
import { Card } from "@/components/ui/card"
import { cn } from "@/lib/utils"
import type { StatusPageData, ServiceStatus } from "./mock-data"
import { generateUptimeHistory } from "./mock-data"

// ── Status badge (needs "success"/"warning" variants not in base shadcn) ──────

function StatusBadge({ variant, className, children }: {
  variant: "success" | "warning" | "destructive" | "secondary" | "outline"
  className?: string; children: React.ReactNode
}) {
  const base = "inline-flex items-center rounded-md border px-2.5 py-0.5 text-xs font-semibold"
  const variants: Record<string, string> = {
    success:     "border-transparent bg-green-600/20 text-green-400",
    warning:     "border-transparent bg-yellow-600/20 text-yellow-400",
    destructive: "border-transparent bg-red-600/20 text-red-400",
    secondary:   "border-transparent bg-secondary text-secondary-foreground",
    outline:     "text-foreground border-border",
  }
  return <span className={cn(base, variants[variant], className)}>{children}</span>
}

// ── Helpers ────────────────────────────────────────────────────────────────────

const STATUS_LABEL_ZH: Record<ServiceStatus, string> = {
  operational: "正常", degraded: "降级", outage: "中断", maintenance: "维护中",
}

function overallStatusConfig(status: ServiceStatus) {
  switch (status) {
    case "operational": return { label: "全部服务正常",  variant: "success"     as const, icon: <CheckCircle2 className="h-6 w-6 text-green-400" />,  bg: "bg-green-900/20 border-green-800" }
    case "degraded":    return { label: "部分服务降级",  variant: "warning"     as const, icon: <AlertTriangle className="h-6 w-6 text-yellow-400" />, bg: "bg-yellow-900/20 border-yellow-800" }
    case "outage":      return { label: "严重服务中断",  variant: "destructive" as const, icon: <XCircle className="h-6 w-6 text-red-400" />,         bg: "bg-red-900/20 border-red-800" }
    case "maintenance": return { label: "计划维护中",    variant: "secondary"   as const, icon: <AlertTriangle className="h-6 w-6 text-blue-400" />,   bg: "bg-blue-900/20 border-blue-800" }
  }
}

function monitorDot(status: ServiceStatus) {
  const colors: Record<ServiceStatus, string> = {
    operational: "bg-green-500", degraded: "bg-yellow-500", outage: "bg-red-500", maintenance: "bg-blue-500",
  }
  return <span aria-label={STATUS_LABEL_ZH[status]} role="img" className={cn("inline-block h-2.5 w-2.5 rounded-full", colors[status])} />
}

function uptimeDayColor(status: ServiceStatus) {
  const colors: Record<ServiceStatus, string> = {
    operational: "bg-green-600", degraded: "bg-yellow-500", outage: "bg-red-600", maintenance: "bg-blue-600",
  }
  return colors[status]
}

function formatDate(iso: string) {
  return new Date(iso).toLocaleString("zh-CN", { year: "numeric", month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", timeZone: "UTC", timeZoneName: "short" })
}

// ── Main component ─────────────────────────────────────────────────────────────

export function StatusClient({ data }: { data: StatusPageData }) {
  const [expandedGroups, setExpandedGroups] = useState(() => new Set(data.groups.map(g => g.id)))
  const [email,          setEmail]           = useState("")
  const [subStatus,      setSubStatus]       = useState<"idle" | "loading" | "success" | "error">("idle")
  const [subError,       setSubError]        = useState("")

  const statusCfg = overallStatusConfig(data.overallStatus)
  const uptimeHistory = useMemo(() => generateUptimeHistory(99.5), [])

  function toggleGroup(id: string) {
    setExpandedGroups(prev => { const n = new Set(prev); n.has(id) ? n.delete(id) : n.add(id); return n })
  }

  async function handleSubscribe(e: React.FormEvent) {
    e.preventDefault(); if (!email.trim()) return
    setSubStatus("loading"); setSubError("")
    try {
      const apiBase = process.env.NEXT_PUBLIC_API_URL ?? ""
      const res = await fetch(`${apiBase}/api/v1/status-pages/${data.slug}/subscribe`, {
        method: "POST", headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ channel_type: "email", endpoint: email.trim(), events: ["incident", "recovery"] }),
      })
      if (!res.ok) { const j = await res.json().catch(() => ({})); setSubError((j as any)?.error?.message ?? "订阅失败"); setSubStatus("error"); return }
      setSubStatus("success"); setEmail("")
    } catch { setSubError("网络错误，请重试"); setSubStatus("error") }
  }

  return (
    <div className="min-h-screen bg-background text-foreground">
      <div className="mx-auto max-w-3xl px-4 py-12">

        {/* Header */}
        <div className="mb-10 text-center">
          <h1 className="mb-4 text-3xl font-bold tracking-tight" data-testid="status-title">{data.title}</h1>
          <div className={cn("inline-flex items-center gap-3 rounded-xl border px-6 py-3", statusCfg.bg)} data-testid="overall-status">
            {statusCfg.icon}
            <StatusBadge variant={statusCfg.variant} className="px-3 py-1 text-sm">{statusCfg.label}</StatusBadge>
          </div>
        </div>

        {/* Service Groups */}
        <div className="mb-10 space-y-4" data-testid="service-groups">
          {data.groups.map(group => {
            const expanded = expandedGroups.has(group.id)
            return (
              <Card key={group.id}>
                <button className="flex w-full items-center justify-between px-5 py-4 text-left transition-colors hover:bg-white/5"
                  onClick={() => toggleGroup(group.id)} aria-expanded={expanded} data-testid={`group-toggle-${group.id}`}>
                  <span className="text-base font-semibold">{group.name}</span>
                  {expanded ? <ChevronDown className="h-4 w-4 text-muted-foreground" /> : <ChevronRight className="h-4 w-4 text-muted-foreground" />}
                </button>
                {expanded && (
                  <div className="divide-y divide-border border-t border-border">
                    {group.monitors.map(monitor => (
                      <div key={monitor.id} className="flex items-center justify-between px-5 py-3" data-testid={`monitor-row-${monitor.id}`}>
                        <span className="text-sm">{monitor.name}</span>
                        <div className="flex items-center gap-2">
                          <span className="text-xs text-muted-foreground">{monitor.uptimePercent.toFixed(2)}%</span>
                          {monitorDot(monitor.status)}
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </Card>
            )
          })}
        </div>

        {/* 90-Day Uptime */}
        <div className="mb-10" data-testid="uptime-history">
          <h2 className="mb-4 text-lg font-semibold">历史可用率（过去 90 天）</h2>
          <Card className="p-5">
            <div className="grid gap-0.5" style={{ gridTemplateColumns: "repeat(90, 1fr)" }} aria-label="90天可用率方块图" data-testid="uptime-grid">
              {uptimeHistory.map((day, i) => (
                <div key={i} title={`${day.date}: ${day.uptime.toFixed(1)}%`}
                  aria-label={`${day.date} 可用率 ${day.uptime.toFixed(1)}%，状态：${STATUS_LABEL_ZH[day.status]}`}
                  className={cn("h-5 w-full rounded-sm", uptimeDayColor(day.status))} />
              ))}
            </div>
            <div className="mt-3 flex items-center justify-between text-xs text-muted-foreground">
              <span>90 天前</span>
              <div className="flex items-center gap-3">
                {[{ cls: "bg-green-600", label: "正常" }, { cls: "bg-yellow-500", label: "降级" }, { cls: "bg-red-600", label: "中断" }].map(({ cls, label }) => (
                  <span key={label} className="flex items-center gap-1">
                    <span className={cn("inline-block h-2.5 w-2.5 rounded-sm", cls)} />{label}
                  </span>
                ))}
              </div>
              <span>今天</span>
            </div>
          </Card>
        </div>

        {/* Recent Events */}
        <div className="mb-10" data-testid="recent-events">
          <h2 className="mb-4 text-lg font-semibold">最近事件公告</h2>
          {data.events.length === 0 ? (
            <Card className="px-5 py-8 text-center text-sm text-muted-foreground">最近 90 天内无事件记录</Card>
          ) : (
            <div className="space-y-3">
              {data.events.map(evt => (
                <Card key={evt.id} className="px-5 py-4">
                  <div className="mb-2 flex items-start justify-between gap-4">
                    <h3 className="text-sm font-medium">{evt.title}</h3>
                    <StatusBadge variant={evt.status === "resolved" ? "success" : "warning"}>
                      {evt.status === "resolved" ? "已解决" : "处理中"}
                    </StatusBadge>
                  </div>
                  <p className="mb-3 text-xs leading-relaxed text-muted-foreground">{evt.description}</p>
                  <div className="space-y-1 text-xs text-muted-foreground">
                    <div>发生时间：{formatDate(evt.createdAt)}</div>
                    {evt.resolvedAt && <div>解决时间：{formatDate(evt.resolvedAt)}</div>}
                    <div>影响服务：{evt.affectedServices.join("、")}</div>
                  </div>
                </Card>
              ))}
            </div>
          )}
        </div>

        {/* Subscribe */}
        <div className="mb-10" data-testid="subscribe-section">
          <h2 className="mb-4 text-lg font-semibold">订阅状态更新</h2>
          <Card className="px-5 py-5">
            {subStatus === "success" ? (
              <div className="rounded-md border border-green-600/30 bg-green-600/10 px-4 py-3 text-sm text-green-400" role="alert">
                验证邮件已发送，请查收并点击链接完成订阅。
              </div>
            ) : (
              <form onSubmit={handleSubscribe} className="flex gap-2">
                <input type="email" placeholder="your@email.com" value={email} onChange={e => setEmail(e.target.value)} required
                  className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring" />
                <button type="submit" disabled={subStatus === "loading"}
                  className="inline-flex h-9 items-center justify-center rounded-md bg-primary px-4 text-sm font-medium text-primary-foreground shadow transition-colors hover:bg-primary/90 disabled:pointer-events-none disabled:opacity-50">
                  {subStatus === "loading" ? "发送中…" : "订阅"}
                </button>
              </form>
            )}
            {subStatus === "error" && <p className="mt-2 text-xs text-destructive">{subError}</p>}
          </Card>
        </div>

        {/* Footer */}
        {data.showBranding && (
          <footer className="text-center" data-testid="powered-by">
            <a href="https://idcd.com" target="_blank" rel="noopener noreferrer"
              className="inline-flex items-center gap-1.5 text-xs text-muted-foreground transition-colors hover:text-foreground">
              Powered by idcd <ExternalLink className="h-3 w-3" />
            </a>
          </footer>
        )}
      </div>
    </div>
  )
}
