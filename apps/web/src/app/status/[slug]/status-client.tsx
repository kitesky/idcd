"use client"

import { useMemo, useState } from "react"
import { useTranslations } from "next-intl"
import { ChevronDown, ChevronRight, ExternalLink } from "lucide-react"
import { Card } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { cn } from "@/lib/utils"
import { OverallBanner } from "@/components/status/overall-banner"
import { UptimeBar } from "@/components/status/uptime-bar"
import type { StatusPageData, ServiceStatus, MonitorHistory } from "./types"

// ── Helpers ────────────────────────────────────────────────────────────────────

function monitorDot(status: ServiceStatus, statusLabel: string) {
  const colors: Record<ServiceStatus, string> = {
    operational: "bg-success", degraded: "bg-warning", outage: "bg-destructive", maintenance: "bg-info",
  }
  return <span aria-label={statusLabel} role="img" className={cn("inline-block h-2.5 w-2.5 rounded-full", colors[status])} />
}

function buildAggregateHistory(groups: StatusPageData["groups"]): MonitorHistory[] {
  if (groups.length === 0 || groups[0]!.monitors.length === 0) {
    const now = new Date()
    return Array.from({ length: 30 }, (_, i) => {
      const d = new Date(now); d.setDate(d.getDate() - (29 - i))
      return { date: d.toISOString().slice(0, 10), status: "operational" as ServiceStatus, uptime: 100 }
    })
  }
  const allMonitors = groups.flatMap(g => g.monitors)
  if (allMonitors.length === 0) return []
  const refHistory = allMonitors[0]!.history
  return refHistory.map((day, i) => {
    const dayStatuses = allMonitors.map(m => m.history[i]?.status ?? "operational")
    let worst: ServiceStatus = "operational"
    for (const s of dayStatuses) {
      if (s === "outage") { worst = "outage"; break }
      if (s === "degraded") worst = "degraded"
    }
    const avgUptime = allMonitors.reduce((sum, m) => sum + (m.history[i]?.uptime ?? 100), 0) / allMonitors.length
    return { date: day.date, status: worst, uptime: avgUptime }
  })
}

// ── Main component ─────────────────────────────────────────────────────────────

export function StatusClient({ data }: { data: StatusPageData }) {
  const t = useTranslations("status.page")
  const [expandedGroups, setExpandedGroups] = useState(() => new Set(data.groups.map(g => g.id)))
  const [email,          setEmail]           = useState("")
  const [subStatus,      setSubStatus]       = useState<"idle" | "loading" | "success" | "error">("idle")
  const [subError,       setSubError]        = useState("")

  const aggregateHistory = useMemo(() => buildAggregateHistory(data.groups), [data.groups])

  const statusLabel = (s: ServiceStatus) => t(`statusLabel.${s}`)

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
      if (!res.ok) { const j = await res.json().catch(() => ({})); setSubError((j as { error?: { message?: string } })?.error?.message ?? t("subscribe.failed")); setSubStatus("error"); return }
      setSubStatus("success"); setEmail("")
    } catch { setSubError(t("subscribe.networkError")); setSubStatus("error") }
  }

  return (
    <div className="min-h-screen bg-background text-foreground">
      <div className="mx-auto max-w-3xl px-4 py-12">

        {/* Header */}
        <OverallBanner title={data.title} status={data.overall_status} />

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
                          <span className="text-xs text-muted-foreground">{monitor.uptime_percent.toFixed(2)}%</span>
                          {monitorDot(monitor.status, statusLabel(monitor.status))}
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </Card>
            )
          })}
        </div>

        {/* 30-Day Uptime */}
        <div className="mb-10">
          <UptimeBar history={aggregateHistory} label={t("uptime.title")} showLegend />
        </div>

        {/* Recent Events */}
        <div className="mb-10" data-testid="recent-events">
          <h2 className="mb-4 text-lg font-semibold">{t("recentEvents.title")}</h2>
          <Card className="px-5 py-8 text-center text-sm text-muted-foreground">{t("recentEvents.empty")}</Card>
        </div>

        {/* Subscribe */}
        <div className="mb-10" data-testid="subscribe-section">
          <h2 className="mb-4 text-lg font-semibold">{t("subscribe.title")}</h2>
          <Card className="px-5 py-5">
            {subStatus === "success" ? (
              <Alert className="border-success/30 bg-success/10 text-success">
                <AlertDescription>{t("subscribe.successDesc")}</AlertDescription>
              </Alert>
            ) : (
              <form onSubmit={handleSubscribe} className="flex gap-2">
                <Input
                  type="email"
                  placeholder={t("subscribe.emailPlaceholder")}
                  value={email}
                  onChange={e => setEmail(e.target.value)}
                  required
                  className="h-9"
                />
                <Button type="submit" disabled={subStatus === "loading"} size="sm">
                  {subStatus === "loading" ? t("subscribe.submitting") : t("subscribe.submit")}
                </Button>
              </form>
            )}
            {subStatus === "error" && <p className="mt-2 text-xs text-destructive">{subError}</p>}
          </Card>
        </div>

        {/* Footer */}
        {data.branding && (
          <footer className="text-center" data-testid="powered-by">
            <a href="https://idcd.com" target="_blank" rel="noopener noreferrer"
              className="inline-flex items-center gap-1.5 text-xs text-muted-foreground transition-colors hover:text-foreground">
              {t("footer.poweredBy")} <ExternalLink className="h-3 w-3" />
            </a>
          </footer>
        )}
      </div>
    </div>
  )
}
