"use client"

import { useState, useEffect } from "react"
import Link from "next/link"
import { useRouter } from "next/navigation"
import { useTranslations, useLocale } from "next-intl"
import { bcp47Of } from "@/i18n/registry"
import {
  ArrowLeft,
  Bell,
  ChevronDown,
  Edit,
  Pause,
  Play,
  Plus,
  Trash2,
  Wifi,
  Radio,
} from "lucide-react"
import { toast } from "sonner"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible"
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Skeleton } from "@/components/ui/skeleton"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { type Monitor, type MonitorStatus, TYPE_LABELS } from "../types"
import { apiRequest, API_BASE } from "@/lib/api"

interface LatestCheck {
  monitor_id: string
  node_id: string
  status: "up" | "down" | "degraded"
  latency_ms: number | null
  checked_at: string
  error: string
}

interface AlertEvent {
  id: string
  monitor_name: string
  status: "firing" | "resolved"
  fired_at: string
  resolved_at?: string
}

interface PolicyItem {
  id: string
  name: string
  enabled: boolean
}

interface CheckBucket {
  bucket_start: string
  total: number
  success: number
  failure: number
  avg_latency_ms: number
  status: "up" | "down" | "degraded" | "empty"
}

function checkStatusBadge(status: "up" | "down" | "degraded", degradedLabel: string) {
  switch (status) {
    case "up":
      return <Badge variant="success">UP</Badge>
    case "down":
      return <Badge variant="destructive">DOWN</Badge>
    case "degraded":
      return <Badge variant="warning">{degradedLabel}</Badge>
  }
}

function statusBadge(status: MonitorStatus, degradedLabel: string) {
  switch (status) {
    case "UP":
      return <Badge variant="success">UP</Badge>
    case "DOWN":
      return <Badge variant="destructive">DOWN</Badge>
    case "PAUSED":
      return <Badge variant="secondary">PAUSED</Badge>
    case "degraded":
      return <Badge variant="warning">{degradedLabel}</Badge>
  }
}

/**
 * Format ISO timestamp using the active locale (BCP 47). We resolve the
 * registry short code via `useLocale()` and convert with `bcp47Of(...)` —
 * we never pass the short code directly to Intl.* APIs.
 */
function makeFormatDateTime(bcp47: string) {
  return function formatDateTime(iso: string): string {
    return new Date(iso).toLocaleString(bcp47, {
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
    })
  }
}

interface EditMonitorDialogProps {
  monitor: Monitor
  onUpdated: (m: Monitor) => void
}

function EditMonitorDialog({ monitor, onUpdated }: EditMonitorDialogProps) {
  const t = useTranslations("monitors")
  const [open, setOpen] = useState(false)
  const [name, setName] = useState(monitor.name)
  const [_target, setTarget] = useState(monitor.target)
  const [intervalSeconds, setIntervalSeconds] = useState(monitor.intervalSeconds)
  const [advancedOpen, setAdvancedOpen] = useState(false)
  // Advanced config fields
  const [timeoutMs, setTimeoutMs] = useState<number | "">(monitor.timeoutMs ?? "")
  const [assertStatusCode, setAssertStatusCode] = useState<number | "">(monitor.assertStatusCode ?? "")
  const [keywordMatch, setKeywordMatch] = useState(monitor.keywordMatch ?? "")
  const [packetLossThreshold, setPacketLossThreshold] = useState<number | "">(monitor.packetLossThreshold ?? "")
  const [port, setPort] = useState<number | "">(monitor.port ?? "")
  const [expectedIp, setExpectedIp] = useState(monitor.expectedIp ?? "")
  const [sslExpiryDays, setSslExpiryDays] = useState<number | "">(monitor.sslExpiryDays ?? "")
  const [submitting, setSubmitting] = useState(false)

  function handleOpenChange(v: boolean) {
    if (v) {
      // reset fields to current monitor values when opening
      setName(monitor.name)
      setTarget(monitor.target)
      setIntervalSeconds(monitor.intervalSeconds)
      setTimeoutMs(monitor.timeoutMs ?? "")
      setAssertStatusCode(monitor.assertStatusCode ?? "")
      setKeywordMatch(monitor.keywordMatch ?? "")
      setPacketLossThreshold(monitor.packetLossThreshold ?? "")
      setPort(monitor.port ?? "")
      setExpectedIp(monitor.expectedIp ?? "")
      setSslExpiryDays(monitor.sslExpiryDays ?? "")
      setAdvancedOpen(false)
    }
    setOpen(v)
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setSubmitting(true)
    try {
      // Pack config fields into a config JSON object (matching API schema)
      const config: Record<string, unknown> = {}
      if (timeoutMs !== "") config.timeout_ms = timeoutMs
      if (assertStatusCode !== "") config.assert_status_code = assertStatusCode
      if (keywordMatch) config.keyword = keywordMatch
      if (packetLossThreshold !== "") config.packet_loss_threshold = packetLossThreshold
      if (port !== "") config.port = port
      if (expectedIp) config.expected_ip = expectedIp
      if (sslExpiryDays !== "") config.expiry_warning_days = sslExpiryDays

      const body: Record<string, unknown> = { name, interval_s: intervalSeconds }
      if (Object.keys(config).length > 0) body.config = config

      const json = await apiRequest<{ data: Record<string, unknown> }>(`/v1/monitors/${monitor.id}`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      })
      // Prefer server response; fall back to optimistic update
      const rawCfg = (json?.data?.config as Record<string, unknown> | null) ?? config
      const updated: Monitor = {
        ...monitor,
        name,
        intervalSeconds,
        timeoutMs: rawCfg.timeout_ms as number | undefined,
        assertStatusCode: rawCfg.assert_status_code as number | undefined,
        keywordMatch: (rawCfg.keyword ?? rawCfg.keyword_match) as string | undefined,
        packetLossThreshold: rawCfg.packet_loss_threshold as number | undefined,
        port: rawCfg.port as number | undefined,
        expectedIp: rawCfg.expected_ip as string | undefined,
        sslExpiryDays: (rawCfg.expiry_warning_days ?? rawCfg.ssl_expiry_days) as number | undefined,
      }
      onUpdated(updated)
      setOpen(false)
      toast.success(t("detail.monitorUpdated"))
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : t("error.updateFailed")
      toast.error(msg)
    } finally {
      setSubmitting(false)
    }
  }

  // Determine which advanced fields are relevant for this monitor type
  const showAssertStatus = ["http", "https", "keyword", "llm_endpoint", "tool_api", "rag"].includes(monitor.type)
  const showKeyword = monitor.type === "keyword"
  const showTimeout = ["http", "https", "ping", "tcp", "keyword", "llm_endpoint", "tool_api", "rag"].includes(monitor.type)
  const showPacketLoss = monitor.type === "ping"
  const showPort = monitor.type === "tcp"
  const showExpectedIp = monitor.type === "dns"
  const showSslExpiry = monitor.type === "ssl_expiry"

  const hasAdvancedFields = showAssertStatus || showKeyword || showTimeout || showPacketLoss || showPort || showExpectedIp || showSslExpiry

  return (
    <>
      <Button variant="outline" size="sm" onClick={() => handleOpenChange(true)}>
        <Edit className="mr-2 h-4 w-4" />
        {t("actions.edit")}
      </Button>
      <Dialog open={open} onOpenChange={handleOpenChange}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t("edit.title")}</DialogTitle>
          </DialogHeader>
          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-1.5">
              <Label htmlFor="edit-name">{t("edit.name")}</Label>
              <Input
                id="edit-name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                required
                placeholder={t("edit.name")}
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="edit-interval">{t("edit.interval")}</Label>
              <Input
                id="edit-interval"
                type="number"
                min={10}
                value={intervalSeconds}
                onChange={(e) => setIntervalSeconds(Number(e.target.value))}
                required
              />
            </div>

            {hasAdvancedFields && (
              <Collapsible open={advancedOpen} onOpenChange={setAdvancedOpen}>
                <CollapsibleTrigger asChild>
                  <Button type="button" variant="ghost" size="sm" className="w-full justify-between px-0 text-xs text-muted-foreground hover:text-foreground">
                    {t("edit.advancedConfig")}
                    <ChevronDown className={`h-3.5 w-3.5 transition-transform ${advancedOpen ? "rotate-180" : ""}`} />
                  </Button>
                </CollapsibleTrigger>
                <CollapsibleContent className="space-y-3 pt-2">
                  {showTimeout && (
                    <div className="space-y-1.5">
                      <Label htmlFor="edit-timeout" className="text-xs">{t("edit.timeout")}</Label>
                      <Input
                        id="edit-timeout"
                        type="number"
                        min={100}
                        placeholder={t("edit.placeholderTimeout")}
                        value={timeoutMs}
                        onChange={(e) => setTimeoutMs(e.target.value === "" ? "" : Number(e.target.value))}
                        className="text-xs"
                      />
                    </div>
                  )}
                  {showAssertStatus && (
                    <div className="space-y-1.5">
                      <Label htmlFor="edit-assert" className="text-xs">{t("edit.assertStatus")}</Label>
                      <Input
                        id="edit-assert"
                        type="number"
                        placeholder={t("edit.placeholderAssertStatus")}
                        value={assertStatusCode}
                        onChange={(e) => setAssertStatusCode(e.target.value === "" ? "" : Number(e.target.value))}
                        className="text-xs"
                      />
                    </div>
                  )}
                  {showKeyword && (
                    <div className="space-y-1.5">
                      <Label htmlFor="edit-keyword" className="text-xs">{t("edit.keywordMatch")}</Label>
                      <Input
                        id="edit-keyword"
                        placeholder={t("edit.keywordPlaceholder")}
                        value={keywordMatch}
                        onChange={(e) => setKeywordMatch(e.target.value)}
                        className="text-xs"
                      />
                    </div>
                  )}
                  {showPacketLoss && (
                    <div className="space-y-1.5">
                      <Label htmlFor="edit-packetloss" className="text-xs">{t("edit.packetLoss")}</Label>
                      <Input
                        id="edit-packetloss"
                        type="number"
                        min={0}
                        max={100}
                        placeholder={t("edit.placeholderPacketLoss")}
                        value={packetLossThreshold}
                        onChange={(e) => setPacketLossThreshold(e.target.value === "" ? "" : Number(e.target.value))}
                        className="text-xs"
                      />
                    </div>
                  )}
                  {showPort && (
                    <div className="space-y-1.5">
                      <Label htmlFor="edit-port" className="text-xs">{t("edit.port")}</Label>
                      <Input
                        id="edit-port"
                        type="number"
                        min={1}
                        max={65535}
                        placeholder={t("edit.placeholderPort")}
                        value={port}
                        onChange={(e) => setPort(e.target.value === "" ? "" : Number(e.target.value))}
                        className="text-xs"
                      />
                    </div>
                  )}
                  {showExpectedIp && (
                    <div className="space-y-1.5">
                      <Label htmlFor="edit-expectedip" className="text-xs">{t("edit.expectedIp")}</Label>
                      <Input
                        id="edit-expectedip"
                        placeholder={t("edit.placeholderExpectedIp")}
                        value={expectedIp}
                        onChange={(e) => setExpectedIp(e.target.value)}
                        className="font-mono text-xs"
                      />
                    </div>
                  )}
                  {showSslExpiry && (
                    <div className="space-y-1.5">
                      <Label htmlFor="edit-sslexpiry" className="text-xs">{t("edit.sslExpiryDays")}</Label>
                      <Input
                        id="edit-sslexpiry"
                        type="number"
                        min={1}
                        placeholder={t("edit.placeholderSslDays")}
                        value={sslExpiryDays}
                        onChange={(e) => setSslExpiryDays(e.target.value === "" ? "" : Number(e.target.value))}
                        className="text-xs"
                      />
                    </div>
                  )}
                </CollapsibleContent>
              </Collapsible>
            )}

            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setOpen(false)} disabled={submitting}>
                {t("bulk.cancel")}
              </Button>
              <Button type="submit" disabled={submitting}>
                {submitting ? t("edit.saving") : t("edit.save")}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </>
  )
}

interface MonitorDetailClientProps {
  monitor: Monitor | null
  monitorId: string
}

const EMPTY_BUCKETS: CheckBucket[] = Array.from({ length: 48 }, () => ({
  bucket_start: "",
  total: 0,
  success: 0,
  failure: 0,
  avg_latency_ms: 0,
  status: "empty" as const,
}))

export function MonitorDetailClient({ monitor, monitorId }: MonitorDetailClientProps) {
  const router = useRouter()
  const t = useTranslations("monitors")
  const locale = useLocale()
  const formatDateTime = makeFormatDateTime(bcp47Of(locale))
  const degradedLabel = t("status.degraded")
  const [currentMonitor, setCurrentMonitor] = useState<Monitor | null>(monitor)
  const id = monitor?.id ?? monitorId
  const [hoveredBlock, setHoveredBlock] = useState<number | null>(null)
  const [deleteOpen, setDeleteOpen] = useState(false)
  const [deleting, setDeleting] = useState(false)
  // Pause/resume confirm flow: stores the intended action so a single dialog
  // can render both pause and resume copy.
  const [pendingToggle, setPendingToggle] = useState<"pause" | "resume" | null>(null)
  const [togglingStatus, setTogglingStatus] = useState(false)
  const [latestCheck, setLatestCheck] = useState<LatestCheck | null>(null)
  const [checkBuckets, setCheckBuckets] = useState<CheckBucket[] | null>(null)
  const [bucketLoading, setBucketLoading] = useState(true)
  const [alertEvents, setAlertEvents] = useState<AlertEvent[]>([])
  const [alertsLoading, setAlertsLoading] = useState(false)
  const [policies, setPolicies] = useState<PolicyItem[]>([])

  useEffect(() => {
    const url = `${API_BASE}/v1/monitors/${id}/stream`
    const es = new EventSource(url, { withCredentials: true })
    es.addEventListener("check", (e: MessageEvent) => {
      try {
        const data = JSON.parse(e.data)
        if (data?.type === "ping") return
        setLatestCheck(data as LatestCheck)
      } catch {
      }
    })
    es.addEventListener("error", () => {
    })
    return () => es.close()
  }, [id])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- id 变化时需重置 loading，随后异步 fetch
    setAlertsLoading(true)
    apiRequest<{ data: { items: AlertEvent[] } }>(
      `/v1/alert-events?monitor_id=${id}&limit=10`
    )
      .then((json) => {
        const events = json?.data?.items
        setAlertEvents(Array.isArray(events) ? events : [])
      })
      .catch(() => {
        setAlertEvents([])
      })
      .finally(() => {
        setAlertsLoading(false)
      })
  }, [id])

  useEffect(() => {
    apiRequest<{ data: { items: PolicyItem[] } }>(`/v1/alert-policies?monitor_id=${id}`)
      .then((json) => {
        const items = json?.data?.items
        setPolicies(Array.isArray(items) ? items : [])
      })
      .catch(() => {
        setPolicies([])
      })
  }, [id])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- id 变化时需重置 loading，随后异步 fetch
    setBucketLoading(true)
    apiRequest<{ data: { buckets: CheckBucket[] } }>(
      `/v1/monitors/${id}/checks?hours=24`
    )
      .then((json) => {
        const buckets: CheckBucket[] = json?.data?.buckets ?? []
        setCheckBuckets(buckets.length > 0 ? buckets : [])
      })
      .catch(() => {
        setCheckBuckets([])
      })
      .finally(() => {
        setBucketLoading(false)
      })
  }, [id])

  function relativeTime(iso: string): string {
    // eslint-disable-next-line react-hooks/purity -- 相对时间展示需要当前时间
    const diffMs = Date.now() - new Date(iso).getTime()
    const diffS = Math.floor(diffMs / 1000)
    if (diffS < 10) return t("time.justNow")
    if (diffS < 60) return t("time.secondsAgo", { count: diffS })
    const diffM = Math.floor(diffS / 60)
    if (diffM < 60) return t("time.minutesAgo", { count: diffM })
    const diffH = Math.floor(diffM / 60)
    return t("time.hoursAgo", { count: diffH })
  }

  function formatDuration(durationS: number, isOngoing: boolean): string {
    let result: string
    if (durationS < 60) {
      result = t("duration.seconds", { count: durationS })
    } else if (durationS < 3600) {
      result = t("duration.minutes", { count: Math.floor(durationS / 60) })
    } else {
      result = t("duration.hours", { count: Math.floor(durationS / 3600) })
    }
    if (isOngoing) result += t("duration.ongoing")
    return result
  }

  async function handleDelete() {
    setDeleting(true)
    try {
      await apiRequest(`/v1/monitors/${id}`, { method: "DELETE" })
      toast.success(t("detail.monitorDeleted"))
      router.push("/app/monitors")
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : t("error.deleteFailed")
      toast.error(msg)
      setDeleting(false)
      setDeleteOpen(false)
    }
  }

  function requestToggle() {
    if (!currentMonitor) return
    setPendingToggle(currentMonitor.status === "PAUSED" ? "resume" : "pause")
  }

  // Previously this function only mutated client state — the backend never
  // received PATCH /v1/monitors/:id and the monitor kept ticking. Now it confirms
  // first, calls the API, and only updates UI on success (with rollback on error).
  async function confirmToggle() {
    if (!currentMonitor || !pendingToggle) return
    const action = pendingToggle
    setPendingToggle(null)
    setTogglingStatus(true)
    const prevStatus = currentMonitor.status
    const newApiStatus = action === "pause" ? "paused" : "active"
    const newFrontendStatus: Monitor["status"] = action === "pause" ? "PAUSED" : "UP"
    // Optimistic
    setCurrentMonitor({ ...currentMonitor, status: newFrontendStatus })
    try {
      await apiRequest(`/v1/monitors/${id}`, {
        method: "PATCH",
        body: JSON.stringify({ status: newApiStatus }),
      })
      toast.success(action === "pause" ? t("actions.pause") : t("actions.resume"))
    } catch (err: unknown) {
      // Roll back on failure
      setCurrentMonitor((prev) => (prev ? { ...prev, status: prevStatus } : null))
      const msg = err instanceof Error ? err.message : t("error.updateFailed")
      toast.error(msg)
    } finally {
      setTogglingStatus(false)
    }
  }

  if (!currentMonitor) {
    return (
      <div className="flex flex-col items-center gap-4 py-16 text-muted-foreground">
        <p>{t("detail.notFound")}</p>
        <Link href="/app/monitors" className="text-sm underline">{t("backToList")}</Link>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* 顶部：名称 + 状态 + 操作 */}
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-3">
          <Link
            href="/app/monitors"
            className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground transition-colors"
          >
            <ArrowLeft className="h-4 w-4" />
            {t("backToList")}
          </Link>
        </div>
        <div className="flex items-center gap-2">
          <EditMonitorDialog monitor={currentMonitor} onUpdated={setCurrentMonitor} />
          <Button variant="outline" size="sm" asChild>
            <Link href={`/app/alerts?tab=policies&monitor=${encodeURIComponent(currentMonitor.name)}`}>
              <Bell className="mr-2 h-4 w-4" />
              {t("actions.alertPolicy")}
            </Link>
          </Button>
          <Button variant="outline" size="sm" onClick={requestToggle} disabled={togglingStatus}>
            {currentMonitor.status === "PAUSED" ? (
              <>
                <Play className="mr-2 h-4 w-4" />
                {t("actions.resume")}
              </>
            ) : (
              <>
                <Pause className="mr-2 h-4 w-4" />
                {t("actions.pause")}
              </>
            )}
          </Button>
          <Button
            variant="outline"
            size="sm"
            className="text-destructive hover:text-destructive"
            onClick={() => setDeleteOpen(true)}
          >
            <Trash2 className="mr-2 h-4 w-4" />
            {t("actions.delete")}
          </Button>
        </div>

        <AlertDialog open={deleteOpen} onOpenChange={setDeleteOpen}>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>{t("confirm.deleteTitle")}</AlertDialogTitle>
              <AlertDialogDescription>
                {t("confirm.deleteDesc", { name: currentMonitor.name })}
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel disabled={deleting}>{t("bulk.cancel")}</AlertDialogCancel>
              <AlertDialogAction
                onClick={handleDelete}
                disabled={deleting}
                className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              >
                {deleting ? t("confirm.deleting") : t("confirm.confirmDelete")}
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>

        {/* Pause/Resume 确认 Dialog */}
        <AlertDialog
          open={pendingToggle !== null}
          onOpenChange={(open) => {
            if (!open) setPendingToggle(null)
          }}
        >
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>
                {pendingToggle === "pause" ? t("confirm.pauseTitle") : t("confirm.resumeTitle")}
              </AlertDialogTitle>
              <AlertDialogDescription>
                {pendingToggle === "pause"
                  ? t("confirm.pauseDesc", { name: currentMonitor.name })
                  : t("confirm.resumeDesc", { name: currentMonitor.name })}
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>{t("bulk.cancel")}</AlertDialogCancel>
              <AlertDialogAction onClick={confirmToggle}>
                {pendingToggle === "pause"
                  ? t("confirm.confirmPause")
                  : t("confirm.confirmResume")}
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </div>

      {/* 监控名称 + 状态 */}
      <div>
        <div className="flex items-center gap-3 flex-wrap">
          <h1 className="text-3xl font-bold tracking-tight">
            {currentMonitor.name}
          </h1>
          {statusBadge(currentMonitor.status, degradedLabel)}
          <Badge variant="outline">{t(`type.${currentMonitor.type}` as never) || TYPE_LABELS[currentMonitor.type]}</Badge>
        </div>
        <p className="mt-1 font-mono text-sm text-muted-foreground">
          {currentMonitor.target}
        </p>
        {/* 配置摘要 */}
        {(() => {
          const items: string[] = []
          if (currentMonitor.timeoutMs) items.push(t("detail.configTimeout", { value: currentMonitor.timeoutMs }))
          if (currentMonitor.assertStatusCode) items.push(t("detail.configAssert", { value: currentMonitor.assertStatusCode }))
          if (currentMonitor.keywordMatch) items.push(t("detail.configKeyword", { value: currentMonitor.keywordMatch }))
          if (currentMonitor.sslExpiryDays) items.push(t("detail.configSslDays", { value: currentMonitor.sslExpiryDays }))
          if (currentMonitor.packetLossThreshold) items.push(t("detail.configPacketLoss", { value: currentMonitor.packetLossThreshold }))
          if (currentMonitor.port) items.push(t("detail.configPort", { value: currentMonitor.port }))
          if (currentMonitor.expectedIp) items.push(t("detail.configExpectedIp", { value: currentMonitor.expectedIp }))
          if (items.length === 0) return null
          return (
            <div className="flex flex-wrap gap-1.5 mt-2">
              {items.map(item => (
                <Badge key={item} variant="outline" className="text-xs font-mono">{item}</Badge>
              ))}
            </div>
          )
        })()}
      </div>

      <div className="flex items-center gap-3 rounded-md border border-dashed px-4 py-3" data-testid="sse-live-check">
        <Badge variant="secondary" className="gap-1.5">
          <Radio className="h-3 w-3 animate-pulse" />
          {t("detail.liveUpdate")}
        </Badge>
        {latestCheck ? (
          <div className="flex items-center gap-3 text-xs">
            {checkStatusBadge(latestCheck.status, degradedLabel)}
            {latestCheck.latency_ms != null && (
              <span className="font-mono text-muted-foreground">
                {latestCheck.latency_ms}ms
              </span>
            )}
            <span className="text-muted-foreground">
              {relativeTime(latestCheck.checked_at)}
            </span>
          </div>
        ) : (
          <span className="text-xs text-muted-foreground">{t("detail.waitingData")}</span>
        )}
      </div>

      {/* 统计卡片行 */}
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              {t("detail.stats24h")}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p
              className={[
                "text-4xl font-bold tabular-nums",
                currentMonitor.uptimePercent >= 99
                  ? "text-success"
                  : currentMonitor.uptimePercent >= 95
                    ? "text-warning"
                    : "text-destructive",
              ].join(" ")}
            >
              {currentMonitor.uptimePercent.toFixed(1)}%
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              {t("detail.interval")}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-bold tabular-nums">
              {currentMonitor.intervalSeconds < 60
                ? `${currentMonitor.intervalSeconds}s`
                : `${currentMonitor.intervalSeconds / 60}m`}
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="flex items-center gap-2 text-sm font-medium text-muted-foreground">
              <Wifi className="h-4 w-4" />
              {t("detail.concurrentNodes")}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-bold tabular-nums">
              {currentMonitor.concurrentNodes}
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              {t("detail.currentStatus")}
            </CardTitle>
          </CardHeader>
          <CardContent className="pt-3">
            {statusBadge(currentMonitor.status, degradedLabel)}
          </CardContent>
        </Card>
      </div>

      {/* 趋势图：最多 48 块 */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">{t("detail.trend24h")}</CardTitle>
          <p className="text-xs text-muted-foreground mt-0.5">
            {t("detail.trendDesc")}
          </p>
        </CardHeader>
        <CardContent>
          {bucketLoading ? (
            <div className="flex flex-wrap gap-1" data-testid="trend-blocks-loading">
              {Array.from({ length: 48 }).map((_, i) => (
                <Skeleton key={i} className="h-6 w-6 rounded-sm" />
              ))}
            </div>
          ) : (
            <div className="flex flex-wrap gap-1" data-testid="trend-blocks">
              {(checkBuckets && checkBuckets.length > 0 ? checkBuckets : EMPTY_BUCKETS).map(
                (bucket, i) => (
                  <div
                    key={i}
                    data-testid={`trend-block-${i}`}
                    className={[
                      "relative h-6 w-6 rounded-sm border cursor-default transition-transform hover:scale-125",
                      bucket.status === "up"
                        ? "bg-success/20 border-success"
                        : bucket.status === "down"
                          ? "bg-destructive/20 border-destructive"
                          : bucket.status === "degraded"
                            ? "bg-warning/20 border-warning"
                            : "bg-muted border-muted",
                    ].join(" ")}
                    onMouseEnter={() => setHoveredBlock(i)}
                    onMouseLeave={() => setHoveredBlock(null)}
                  >
                    {hoveredBlock === i && (
                      <div className="absolute bottom-full left-1/2 mb-1 -translate-x-1/2 whitespace-nowrap rounded-md bg-popover px-2 py-1 text-xs shadow-md border z-10">
                        {bucket.bucket_start ? (
                          <div className="text-muted-foreground">{formatDateTime(bucket.bucket_start)}</div>
                        ) : (
                          <div className="text-muted-foreground">{t("detail.noData")}</div>
                        )}
                        {bucket.total > 0 && (
                          <>
                            <div className="text-success">{t("detail.checkSuccess", { count: bucket.success })}</div>
                            {bucket.failure > 0 && (
                              <div className="text-destructive">{t("detail.checkFailure", { count: bucket.failure })}</div>
                            )}
                            {bucket.avg_latency_ms > 0 && (
                              <div className="font-mono">{bucket.avg_latency_ms.toFixed(1)}ms</div>
                            )}
                          </>
                        )}
                      </div>
                    )}
                  </div>
                )
              )}
            </div>
          )}
        </CardContent>
      </Card>

      {/* 最近 10 次检测结果（取最新的 10 个非 empty bucket） */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">{t("detail.recentChecks")}</CardTitle>
        </CardHeader>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t("detail.checkTime")}</TableHead>
              <TableHead>{t("detail.checkStatus")}</TableHead>
              <TableHead>{t("detail.checkLatency")}</TableHead>
              <TableHead>{t("detail.checkSuccessFailure")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {bucketLoading ? (
              <TableRow>
                <TableCell colSpan={4} className="text-center text-xs text-muted-foreground py-4">
                  {t("detail.loading")}
                </TableCell>
              </TableRow>
            ) : (() => {
              const rows = [...(checkBuckets ?? [])].reverse().filter((b) => b.status !== "empty").slice(0, 10)
              if (rows.length === 0) return (
                <TableRow>
                  <TableCell colSpan={4} className="text-center text-xs text-muted-foreground py-4">
                    {t("detail.noChecks")}
                  </TableCell>
                </TableRow>
              )
              return rows.map((bucket, i) => (
                  <TableRow key={i}>
                    <TableCell className="font-mono text-xs text-muted-foreground">
                      {bucket.bucket_start ? formatDateTime(bucket.bucket_start) : "-"}
                    </TableCell>
                    <TableCell>
                      {bucket.status === "up" ? (
                        <Badge variant="success">UP</Badge>
                      ) : bucket.status === "down" ? (
                        <Badge variant="destructive">DOWN</Badge>
                      ) : (
                        <Badge variant="warning">{degradedLabel}</Badge>
                      )}
                    </TableCell>
                    <TableCell className="font-mono text-xs">
                      {bucket.avg_latency_ms > 0 ? `${bucket.avg_latency_ms.toFixed(1)}ms` : "-"}
                    </TableCell>
                    <TableCell className="text-xs text-muted-foreground">
                      <span className="text-success">{bucket.success}</span>
                      {" / "}
                      <span className={bucket.failure > 0 ? "text-destructive" : "text-muted-foreground"}>
                        {bucket.failure}
                      </span>
                    </TableCell>
                  </TableRow>
                ))
            })()}
          </TableBody>
        </Table>
      </Card>

      {/* 告警策略 */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle className="text-base">{t("detail.alertPolicies")}</CardTitle>
          <Link
            href={`/app/alerts?tab=policies&monitor=${encodeURIComponent(currentMonitor.name)}`}
            className="text-xs text-muted-foreground hover:text-foreground"
          >
            {t("actions.managePolicy")}
          </Link>
        </CardHeader>
        <CardContent>
          {policies.length === 0 ? (
            <div className="flex items-center justify-between">
              <p className="text-sm text-muted-foreground">{t("detail.noAlertPolicies")}</p>
              <Button size="sm" variant="outline" asChild>
                <Link href={`/app/alerts?tab=policies&monitor=${encodeURIComponent(currentMonitor.name)}`}>
                  <Plus className="mr-1 h-4 w-4" />{t("actions.createPolicy")}
                </Link>
              </Button>
            </div>
          ) : (
            <div className="space-y-2">
              {policies.map(policy => (
                <div key={policy.id} className="flex items-center justify-between text-sm">
                  <span>{policy.name}</span>
                  <Badge variant={policy.enabled ? "secondary" : "outline"}>
                    {policy.enabled ? t("detail.alertFiring") : t("detail.alertResolved")}
                  </Badge>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      {/* 告警历史 */}
      <Card data-testid="alert-history-section">
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle className="text-base">{t("detail.alertHistory")}</CardTitle>
          <Link href="/app/alerts?tab=events" className="text-xs text-muted-foreground hover:text-foreground">
            {t("actions.viewAllAlerts")}
          </Link>
        </CardHeader>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("detail.alertStatus")}</TableHead>
                <TableHead>{t("detail.alertFiredAt")}</TableHead>
                <TableHead>{t("detail.alertDuration")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {alertsLoading ? (
                <TableRow>
                  <TableCell colSpan={3} className="text-center text-xs text-muted-foreground py-4">
                    {t("detail.loading")}
                  </TableCell>
                </TableRow>
              ) : alertEvents.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={3} className="text-center text-xs text-muted-foreground py-4">
                    {t("detail.noAlertHistory")}
                  </TableCell>
                </TableRow>
              ) : (
                alertEvents.map((event) => {
                  const endMs = event.resolved_at
                    ? new Date(event.resolved_at).getTime()
                    // eslint-disable-next-line react-hooks/purity -- 未结束告警的时长展示需要当前时间
                    : Date.now()
                  const durationMs = endMs - new Date(event.fired_at).getTime()
                  const durationS = Math.floor(durationMs / 1000)
                  const duration = formatDuration(durationS, !event.resolved_at)

                  return (
                    <TableRow key={event.id}>
                      <TableCell>
                        {event.status === "firing" ? (
                          <Badge variant="destructive">{t("detail.alertFiring")}</Badge>
                        ) : (
                          <Badge variant="success">{t("detail.alertResolved")}</Badge>
                        )}
                      </TableCell>
                      <TableCell className="font-mono text-xs text-muted-foreground">
                        {relativeTime(event.fired_at)}
                      </TableCell>
                      <TableCell className="text-xs text-muted-foreground">
                        {duration}
                      </TableCell>
                    </TableRow>
                  )
                })
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  )
}
