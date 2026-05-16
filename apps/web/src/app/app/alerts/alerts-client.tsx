"use client"

import { useRef, useState, useEffect, useCallback } from "react"
import { useSearchParams, useRouter, usePathname } from "next/navigation"
import { useTranslations } from "next-intl"
import {
  Bell,
  Mail,
  MessageSquare,
  CheckCheck,
  Plus,
  Trash2,
  Pencil,
  AlertCircle,
  ChevronDown,
  ChevronUp,
  Wifi,
  Loader2,
  CheckCircle2,
  Search,
} from "lucide-react"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardHeader, CardTitle, CardFooter } from "@/components/ui/card"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Checkbox } from "@/components/ui/checkbox"
import { Slider } from "@/components/ui/slider"
import { Skeleton } from "@/components/ui/skeleton"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
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
import {
  Sheet,
  SheetContent,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog"
import { Textarea } from "@/components/ui/textarea"
import { Switch } from "@/components/ui/switch"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { toast } from "sonner"
import {
  type AlertEvent,
  type AlertChannel,
  type AlertPolicy,
  type AlertNotification,
  type AlertSilence,
  type ChannelType,
  CHANNEL_TYPE_LABELS,
  CHANNEL_TYPES,
  formatDuration,
  truncateConfig,
} from "./types"
import { apiRequest } from "@/lib/api"

// ─── Helpers ─────────────────────────────────────────────────────────────────

function statusBadge(status: AlertEvent["status"], t: ReturnType<typeof useTranslations<"alerts">>) {
  if (status === "firing")
    return <Badge variant="destructive">{t("events.status.firing")}</Badge>
  if (status === "resolved")
    return <Badge variant="success">{t("events.status.resolved")}</Badge>
  return <Badge variant="secondary">{t("events.status.acknowledged")}</Badge>
}

function channelIcon(type: ChannelType) {
  if (type === "email") return <Mail className="h-4 w-4" aria-hidden />
  if (type === "wecom" || type === "feishu") return <MessageSquare className="h-4 w-4" aria-hidden />
  return <Bell className="h-4 w-4" aria-hidden />
}

function notifStatusBadge(status: AlertNotification["status"], t: ReturnType<typeof useTranslations<"alerts">>) {
  if (status === "sent") return <Badge variant="success">{t("channels.notifStatus.sent")}</Badge>
  if (status === "failed") return <Badge variant="destructive">{t("channels.notifStatus.failed")}</Badge>
  return <Badge variant="secondary">{t("channels.notifStatus.pending")}</Badge>
}

// ─── Add Channel Form (inside Sheet) ─────────────────────────────────────────

interface AddChannelFormProps {
  onSave: (channel: Omit<AlertChannel, "id" | "verified">) => void
  onCancel: () => void
}

function AddChannelForm({ onSave, onCancel }: AddChannelFormProps) {
  const t = useTranslations("alerts")
  const [type, setType] = useState<ChannelType>("email")
  const [name, setName] = useState("")
  const [config, setConfig] = useState("")

  const configLabel = type === "email" ? t("channels.form.emailLabel") : t("channels.form.webhookLabel")
  const configPlaceholder =
    type === "email" ? t("channels.form.emailPlaceholder") : t("channels.form.webhookPlaceholder")

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim() || !config.trim()) return
    onSave({ name: name.trim(), type, config: config.trim() })
  }

  return (
    <form id="add-channel-form" onSubmit={handleSubmit} className="space-y-5">
      <div className="space-y-2">
        <Label htmlFor="channel-type">{t("channels.form.type")}</Label>
        <Select value={type} onValueChange={(v) => setType(v as ChannelType)}>
          <SelectTrigger id="channel-type">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {CHANNEL_TYPES.map((ct) => (
              <SelectItem key={ct} value={ct}>
                {CHANNEL_TYPE_LABELS[ct]}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      <div className="space-y-2">
        <Label htmlFor="channel-name">{t("channels.form.name")}</Label>
        <Input
          id="channel-name"
          placeholder={t("channels.form.namePlaceholder")}
          value={name}
          onChange={(e) => setName(e.target.value)}
          required
        />
      </div>

      <div className="space-y-2">
        <Label htmlFor="channel-config">{configLabel}</Label>
        <Input
          id="channel-config"
          placeholder={configPlaceholder}
          value={config}
          onChange={(e) => setConfig(e.target.value)}
          required
        />
      </div>
    </form>
  )
}

// ─── Add / Edit Policy Form (inside Sheet) ────────────────────────────────────

interface PolicyFormProps {
  channels: AlertChannel[]
  initial?: AlertPolicy
  onSave: (policy: Omit<AlertPolicy, "id">) => void
  onCancel: () => void
}

function PolicyForm({ channels, initial, onSave, onCancel }: PolicyFormProps) {
  const t = useTranslations("alerts")
  const tCommon = useTranslations("common")
  const [name, setName] = useState(initial?.name ?? "")
  const [monitorName, setMonitorName] = useState(initial?.monitorName ?? "")
  const [selectedChannels, setSelectedChannels] = useState<string[]>(initial?.channelIds ?? [])
  const [delay, setDelay] = useState(initial?.delayMinutes ?? 5)
  const [muteFrom, setMuteFrom] = useState(initial?.muteFrom ?? "")
  const [muteTo, setMuteTo] = useState(initial?.muteTo ?? "")
  const [enabled, setEnabled] = useState(initial?.enabled ?? true)
  const [monitors, setMonitors] = useState<MonitorOption[]>([])
  const [monitorsLoading, setMonitorsLoading] = useState(true)

  useEffect(() => {
    let cancelled = false
    setMonitorsLoading(true)
    apiRequest<{ data: { items: MonitorOption[] } }>("/v1/monitors")
      .then((res) => {
        if (cancelled) return
        const list = res.data.items ?? []
        setMonitors(list)
        // If no initial monitor name is set, default to the first option
        if (!initial?.monitorName && list.length > 0) {
          setMonitorName(list[0]!.name)
        }
      })
      .catch(() => {
        // On error leave monitors empty; select will just show nothing
      })
      .finally(() => {
        if (!cancelled) setMonitorsLoading(false)
      })
    return () => { cancelled = true }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const toggleChannel = (id: string) => {
    setSelectedChannels((prev) =>
      prev.includes(id) ? prev.filter((c) => c !== id) : [...prev, id]
    )
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim()) return
    onSave({
      name: name.trim(),
      monitorName,
      channelIds: selectedChannels,
      delayMinutes: delay,
      muteFrom: muteFrom || undefined,
      muteTo: muteTo || undefined,
      enabled,
    })
  }

  return (
    <form id="policy-form" onSubmit={handleSubmit} className="space-y-5">
      <div className="space-y-2">
        <Label htmlFor="policy-name">{t("policies.form.name")}</Label>
        <Input
          id="policy-name"
          placeholder={t("policies.form.namePlaceholder")}
          value={name}
          onChange={(e) => setName(e.target.value)}
          required
        />
      </div>

      <div className="space-y-2">
        <Label htmlFor="policy-monitor">{t("policies.form.monitor")}</Label>
        <Select value={monitorName} onValueChange={setMonitorName} disabled={monitorsLoading}>
          <SelectTrigger id="policy-monitor" data-testid="policy-monitor-select">
            <SelectValue placeholder={monitorsLoading ? t("policies.form.loading") : t("policies.form.monitorPlaceholder")} />
          </SelectTrigger>
          <SelectContent>
            {monitorsLoading ? (
              <SelectItem value="__loading__" disabled>{t("policies.form.loading")}</SelectItem>
            ) : monitors.length === 0 ? (
              <SelectItem value="__empty__" disabled>{tCommon("noData")}</SelectItem>
            ) : (
              monitors.map((m) => (
                <SelectItem key={m.id} value={m.name}>
                  {m.name}
                </SelectItem>
              ))
            )}
          </SelectContent>
        </Select>
      </div>

      <div className="space-y-2">
        <Label>{t("policies.form.channels")}</Label>
        <Card><CardContent className="space-y-2 p-3">
          {channels.length === 0 && (
            <p className="text-sm text-muted-foreground">{t("policies.form.noChannels")}</p>
          )}
          {channels.map((ch) => (
            <label key={ch.id} className="flex cursor-pointer items-center gap-2">
              <Checkbox
                checked={selectedChannels.includes(ch.id)}
                onCheckedChange={() => toggleChannel(ch.id)}
                aria-label={ch.name}
              />
              <span className="text-sm">{ch.name}</span>
              <span className="ml-auto text-xs text-muted-foreground">
                {CHANNEL_TYPE_LABELS[ch.type]}
              </span>
            </label>
          ))}
        </CardContent></Card>
      </div>

      <div className="space-y-2">
        <Label>{t("policies.form.delay", { min: delay })}</Label>
        <Slider
          min={0}
          max={60}
          step={1}
          value={[delay]}
          onValueChange={([v]) => setDelay(v!)}
          aria-label={t("policies.form.delay", { min: delay })}
        />
        <div className="flex justify-between text-xs text-muted-foreground">
          <span>{t("policies.form.immediately")}</span>
          <span>{t("policies.form.maxDelay")}</span>
        </div>
      </div>

      <div className="space-y-2">
        <Label>{t("policies.form.muteWindow")}</Label>
        <div className="flex items-center gap-3">
          <Input
            type="time"
            value={muteFrom}
            onChange={(e) => setMuteFrom(e.target.value)}
            aria-label={t("policies.form.muteWindow")}
            className="flex-1"
          />
          <span className="text-sm text-muted-foreground">{t("policies.form.muteTo")}</span>
          <Input
            type="time"
            value={muteTo}
            onChange={(e) => setMuteTo(e.target.value)}
            aria-label={t("silences.addDialog.endsAt")}
            className="flex-1"
          />
        </div>
      </div>

      <div className="flex items-center gap-2">
        <Checkbox
          id="policy-enabled"
          checked={enabled}
          onCheckedChange={(v) => setEnabled(Boolean(v))}
        />
        <Label htmlFor="policy-enabled" className="cursor-pointer text-sm">{t("policies.form.enabled")}</Label>
      </div>

    </form>
  )
}

// ─── Events Tab ───────────────────────────────────────────────────────────────

interface EventsTabProps {
  initialMonitorId?: string
}

function EventsTab({ initialMonitorId = "" }: EventsTabProps) {
  const t = useTranslations("alerts")
  const [events, setEvents] = useState<AlertEvent[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  // Filter state — seed monitorId from prop (set once on mount)
  const [status, setStatus] = useState<"" | "firing" | "resolved">("")
  const [monitorId, setMonitorId] = useState(initialMonitorId)
  const [monitors, setMonitors] = useState<MonitorOption[]>([])

  const handleAcknowledge = async (id: string) => {
    try {
      await apiRequest(`/v1/alert-events/${id}/ack`, { method: "POST" })
      setEvents((prev) =>
        prev.map((e) =>
          e.id === id
            ? { ...e, status: "acknowledged" as const, acknowledgedAt: new Date().toISOString() }
            : e
        )
      )
      toast(t("ack.success"))
    } catch (err) {
      toast(err instanceof Error ? err.message : t("ack.failed"))
    }
  }

  // Fetch monitors once for the filter dropdown
  useEffect(() => {
    apiRequest<{ data: { items: MonitorOption[] } }>("/v1/monitors")
      .then((res) => setMonitors(res.data.items ?? []))
      .catch(() => {/* leave monitors empty; filter will just show status only */})
  }, [])

  // Fetch events whenever filters change
  useEffect(() => {
    let cancelled = false
    setLoading(true)
    setError(null)
    const params = new URLSearchParams({ limit: "50" })
    if (status) params.set("status", status)
    if (monitorId) params.set("monitor_id", monitorId)
    apiRequest<{ data: { events: AlertEvent[] } }>(`/v1/alert-events?${params}`)
      .then((res) => { if (!cancelled) setEvents(res.data.events ?? []) })
      .catch((err) => { if (!cancelled) setError(err instanceof Error ? err.message : t("events.loadFailed")) })
      .finally(() => { if (!cancelled) setLoading(false) })
    return () => { cancelled = true }
  }, [status, monitorId])

  const firingCount = events.filter((e) => e.status === "firing").length

  if (loading) {
    return (
      <div className="space-y-4" data-testid="events-skeleton">
        <Skeleton className="h-10 w-full rounded-md" />
        <div className="space-y-3">
          {Array.from({ length: 5 }).map((_, i) => (
            <Skeleton key={i} className="h-10 w-full rounded" />
          ))}
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <Alert variant="destructive" data-testid="events-error">
        <AlertCircle className="h-4 w-4" />
        <AlertTitle>{t("events.loadFailed")}</AlertTitle>
        <AlertDescription>{error}</AlertDescription>
      </Alert>
    )
  }

  return (
    <div className="space-y-4">
      {/* Filter bar */}
      <div className="flex flex-wrap items-center gap-3" data-testid="events-filter-bar">
        <Select
          value={status || "__all__"}
          onValueChange={(v) => setStatus(v === "__all__" ? "" : (v as "firing" | "resolved"))}
        >
          <SelectTrigger className="w-36" data-testid="events-status-filter">
            <SelectValue placeholder={t("events.filter.allStatus")} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="__all__">{t("events.filter.allStatus")}</SelectItem>
            <SelectItem value="firing">{t("events.filter.firing")}</SelectItem>
            <SelectItem value="resolved">{t("events.filter.resolved")}</SelectItem>
          </SelectContent>
        </Select>

        {monitors.length > 0 && (
          <Select
            value={monitorId || "__all__"}
            onValueChange={(v) => setMonitorId(v === "__all__" ? "" : v)}
          >
            <SelectTrigger className="w-44" data-testid="events-monitor-filter">
              <SelectValue placeholder={t("events.filter.allMonitors")} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="__all__">{t("events.filter.allMonitors")}</SelectItem>
              {monitors.map((m) => (
                <SelectItem key={m.id} value={m.id}>{m.name}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        )}

        {(status || monitorId) && (
          <Button
            variant="ghost"
            size="sm"
            onClick={() => { setStatus(""); setMonitorId("") }}
            data-testid="events-clear-filter"
          >
            {t("events.filter.clearFilter")}
          </Button>
        )}
      </div>

      {firingCount > 0 && (
        <Alert variant="destructive" data-testid="firing-alert">
          <AlertCircle className="h-4 w-4" />
          <AlertTitle>{t("events.activeAlert")}</AlertTitle>
          <AlertDescription>
            {t("events.activeAlertDesc", { count: firingCount })}
          </AlertDescription>
        </Alert>
      )}

      {events.length === 0 ? (
        <p className="py-8 text-center text-sm text-muted-foreground" data-testid="events-empty">
          {status || monitorId ? t("events.emptyFiltered") : t("events.empty")}
        </p>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t("events.table.monitorName")}</TableHead>
              <TableHead>{t("events.table.status")}</TableHead>
              <TableHead className="hidden md:table-cell">{t("events.table.startedAt")}</TableHead>
              <TableHead className="hidden md:table-cell">{t("events.table.duration")}</TableHead>
              <TableHead>{t("events.table.actions")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {events.map((evt) => (
              <TableRow key={evt.id} data-testid={`event-row-${evt.id}`}>
                <TableCell className="font-medium">{evt.monitorName}</TableCell>
                <TableCell>{statusBadge(evt.status, t)}</TableCell>
                <TableCell className="hidden md:table-cell text-sm text-muted-foreground">
                  {new Date(evt.startedAt).toLocaleString("zh-CN", {
                    month: "numeric",
                    day: "numeric",
                    hour: "2-digit",
                    minute: "2-digit",
                  })}
                </TableCell>
                <TableCell className="hidden md:table-cell text-sm text-muted-foreground tabular-nums">
                  {formatDuration(
                    evt.startedAt,
                    evt.resolvedAt ?? evt.acknowledgedAt
                  )}
                </TableCell>
                <TableCell>
                  {evt.status === "firing" && (
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => handleAcknowledge(evt.id)}
                      data-testid={`ack-btn-${evt.id}`}
                    >
                      <CheckCheck className="mr-1 h-3 w-3" />
                      {t("ack.label")}
                    </Button>
                  )}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
    </div>
  )
}

// ─── Channels Tab ─────────────────────────────────────────────────────────────

interface ChannelsTabProps {
  channels: AlertChannel[]
  testingIds: Set<string>
  onTest: (id: string) => void
  onDelete: (id: string) => void
  onAdd: () => void
}

interface NotificationsResponse {
  data: { notifications: AlertNotification[] }
}

function ChannelDeliveryHistory({ channelId }: { channelId: string }) {
  const t = useTranslations("alerts")
  const [open, setOpen] = useState(false)
  const [notifications, setNotifications] = useState<AlertNotification[]>([])
  const [notifLoading, setNotifLoading] = useState(false)
  const fetchedRef = useRef(false)

  const handleToggle = useCallback(async () => {
    const willOpen = !open
    setOpen(willOpen)
    if (willOpen && !fetchedRef.current) {
      fetchedRef.current = true
      setNotifLoading(true)
      try {
        const res = await apiRequest<NotificationsResponse>(
          `/v1/alert-channels/${channelId}/notifications?limit=20`
        )
        setNotifications(res.data.notifications)
      } catch {
        // On error keep empty list; already set above
      } finally {
        setNotifLoading(false)
      }
    }
  }, [open, channelId])

  return (
    <div className="border-t">
      <Button
        variant="ghost"
        size="sm"
        className="w-full justify-between rounded-none px-4 py-2 text-xs text-muted-foreground"
        onClick={handleToggle}
        data-testid={`delivery-history-toggle-${channelId}`}
      >
        {t("channels.deliveryHistory", { count: notifications.length })}
        {open ? <ChevronUp className="h-3 w-3" /> : <ChevronDown className="h-3 w-3" />}
      </Button>
      {open && (
        <div data-testid={`delivery-history-content-${channelId}`} className="px-4 pb-3">
          {notifLoading ? (
            <p className="py-2 text-center text-xs text-muted-foreground">{t("policies.form.loading")}</p>
          ) : notifications.length === 0 ? (
            <p className="py-2 text-center text-xs text-muted-foreground">{t("channels.noDeliveryHistory")}</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="text-xs">{t("channels.notifTable.status")}</TableHead>
                  <TableHead className="text-xs">{t("channels.notifTable.eventId")}</TableHead>
                  <TableHead className="text-xs">{t("channels.notifTable.sentAt")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {notifications.slice(0, 10).map((n) => (
                  <TableRow key={n.id} data-testid={`notif-row-${n.id}`}>
                    <TableCell className="py-1">{notifStatusBadge(n.status, t)}</TableCell>
                    <TableCell className="py-1 font-mono text-xs text-muted-foreground">
                      {n.alert_event_id}
                    </TableCell>
                    <TableCell className="py-1 text-xs text-muted-foreground tabular-nums">
                      {n.sent_at
                        ? new Date(n.sent_at).toLocaleString("zh-CN", {
                            month: "numeric",
                            day: "numeric",
                            hour: "2-digit",
                            minute: "2-digit",
                          })
                        : "—"}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </div>
      )}
    </div>
  )
}

function ChannelsTab({ channels, testingIds, onTest, onDelete, onAdd }: ChannelsTabProps) {
  const t = useTranslations("alerts")
  const [channelSearch, setChannelSearch] = useState("")

  const filteredChannels = channels.filter((c) =>
    c.name.toLowerCase().includes(channelSearch.toLowerCase())
  )

  const sortedChannels = [...filteredChannels].sort((a, b) => {
    if (a.verified === b.verified) return 0
    return a.verified ? 1 : -1 // unverified first
  })

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3">
        <div className="relative flex-1">
          <Search className="absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            placeholder={t("channels.search")}
            value={channelSearch}
            onChange={(e) => setChannelSearch(e.target.value)}
            className="pl-8"
            data-testid="channel-search-input"
          />
        </div>
        <Button size="sm" onClick={onAdd} data-testid="add-channel-btn">
          <Plus className="mr-1 h-4 w-4" />
          {t("channels.create")}
        </Button>
      </div>

      <p className="text-sm text-muted-foreground">
        {channelSearch
          ? t("channels.countWithFilter", { filtered: sortedChannels.length, total: channels.length })
          : t("channels.countAll", { count: channels.length })}
      </p>

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {sortedChannels.map((ch) => {
          const isTesting = testingIds.has(ch.id)
          return (
            <Card key={ch.id} data-testid={`channel-card-${ch.id}`}>
              <CardHeader className="pb-3">
                <CardTitle className="flex items-center gap-2 text-sm font-semibold">
                  {channelIcon(ch.type)}
                  <span className="flex-1 truncate">{ch.name}</span>
                  {ch.verified ? (
                    <Badge variant="success" className="shrink-0 gap-1">
                      <CheckCircle2 className="h-3 w-3" />
                      {t("channels.verified")}
                    </Badge>
                  ) : (
                    <Badge variant="warning" className="shrink-0">
                      {t("channels.unverified")}
                    </Badge>
                  )}
                </CardTitle>
                <p className="text-xs text-muted-foreground">
                  {CHANNEL_TYPE_LABELS[ch.type]}
                  {!ch.verified && (
                    <span className="ml-1 text-amber-600 dark:text-amber-400">
                      {t("channels.unverifiedHint")}
                    </span>
                  )}
                </p>
              </CardHeader>
              <CardContent className="pb-2">
                <p className="truncate font-mono text-xs text-muted-foreground">
                  {truncateConfig(ch.config)}
                </p>
              </CardContent>
              <CardFooter className="flex gap-2 pt-0">
                <Button
                  variant="ghost"
                  size="icon"
                  onClick={() => onTest(ch.id)}
                  disabled={isTesting}
                  data-testid={`test-channel-btn-${ch.id}`}
                  aria-label={`${t("channels.create")} ${ch.name}`}
                >
                  {isTesting
                    ? <Loader2 className="h-4 w-4 animate-spin" />
                    : <Wifi className="h-4 w-4" />}
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  className="flex-1 text-destructive hover:bg-destructive/10"
                  onClick={() => onDelete(ch.id)}
                  data-testid={`delete-channel-btn-${ch.id}`}
                  aria-label={`${t("channels.delete.title")} ${ch.name}`}
                >
                  <Trash2 className="h-4 w-4" />
                </Button>
              </CardFooter>
              <ChannelDeliveryHistory channelId={ch.id} />
            </Card>
          )
        })}
      </div>
    </div>
  )
}

// ─── Policies Tab ─────────────────────────────────────────────────────────────

interface PoliciesTabProps {
  policies: AlertPolicy[]
  channels: AlertChannel[]
  onToggle: (id: string) => void
  onEdit: (policy: AlertPolicy) => void
  onDelete: (id: string) => void
  onAdd: () => void
}

function PoliciesTab({
  policies,
  channels,
  onToggle,
  onEdit,
  onDelete,
  onAdd,
}: PoliciesTabProps) {
  const t = useTranslations("alerts")
  const channelName = (id: string) =>
    channels.find((c) => c.id === id)?.name ?? id

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <p className="text-sm text-muted-foreground">{t("policies.count", { count: policies.length })}</p>
        <Button size="sm" onClick={onAdd} data-testid="add-policy-btn">
          <Plus className="mr-1 h-4 w-4" />
          {t("policies.create")}
        </Button>
      </div>

      <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t("policies.table.name")}</TableHead>
              <TableHead className="hidden md:table-cell">{t("policies.table.monitor")}</TableHead>
              <TableHead className="hidden md:table-cell">{t("policies.table.channels")}</TableHead>
              <TableHead className="hidden md:table-cell">{t("policies.table.delay")}</TableHead>
              <TableHead className="hidden md:table-cell">{t("policies.table.muteWindow")}</TableHead>
              <TableHead>{t("policies.table.status")}</TableHead>
              <TableHead>{t("policies.table.actions")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {policies.map((pol) => (
              <TableRow key={pol.id} data-testid={`policy-row-${pol.id}`}>
                <TableCell className="font-medium">{pol.name}</TableCell>
                <TableCell className="hidden md:table-cell text-sm text-muted-foreground">
                  {pol.monitorName}
                </TableCell>
                <TableCell className="hidden md:table-cell">
                  <div className="flex flex-wrap gap-1">
                    {pol.channelIds.map((cid) => (
                      <Badge key={cid} variant="outline" className="text-xs">
                        {channelName(cid)}
                      </Badge>
                    ))}
                    {pol.channelIds.length === 0 && (
                      <span className="text-xs text-muted-foreground">—</span>
                    )}
                  </div>
                </TableCell>
                <TableCell className="hidden md:table-cell text-sm text-muted-foreground tabular-nums">
                  {pol.delayMinutes === 0 ? t("policies.delay.immediately") : t("policies.delay.minutes", { min: pol.delayMinutes })}
                </TableCell>
                <TableCell className="hidden md:table-cell text-sm text-muted-foreground tabular-nums">
                  {pol.muteFrom && pol.muteTo
                    ? `${pol.muteFrom}–${pol.muteTo}`
                    : "—"}
                </TableCell>
                <TableCell>
                  <div className="flex items-center gap-2">
                    <Switch
                      checked={pol.enabled}
                      onCheckedChange={() => onToggle(pol.id)}
                      aria-label={`${pol.name} ${pol.enabled ? t("policies.status.enabled") : t("policies.status.disabled")}`}
                      data-testid={`policy-toggle-${pol.id}`}
                    />
                    <span className="text-xs text-muted-foreground">
                      {pol.enabled ? t("policies.status.enabled") : t("policies.status.disabled")}
                    </span>
                  </div>
                </TableCell>
                <TableCell>
                  <div className="flex items-center gap-1">
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={() => onEdit(pol)}
                      aria-label={`${t("policies.editSheet")} ${pol.name}`}
                      data-testid={`edit-policy-btn-${pol.id}`}
                    >
                      <Pencil className="h-4 w-4" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="text-destructive hover:bg-destructive/10"
                      onClick={() => onDelete(pol.id)}
                      aria-label={`${t("policies.delete.title")} ${pol.name}`}
                      data-testid={`delete-policy-btn-${pol.id}`}
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </div>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
    </div>
  )
}

// ─── Silences Tab ────────────────────────────────────────────────────────────

interface SilencesTabProps {
  silences: AlertSilence[]
  onDelete: (id: string) => void
  onAdd: () => void
}

function SilencesTab({ silences, onDelete, onAdd }: SilencesTabProps) {
  const t = useTranslations("alerts")

  function silenceStatusBadge(status: AlertSilence["status"]) {
    if (status === "active") return <Badge variant="destructive">{t("silences.status.active")}</Badge>
    if (status === "upcoming") return <Badge variant="secondary">{t("silences.status.upcoming")}</Badge>
    return <Badge variant="outline">{t("silences.status.expired")}</Badge>
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <p className="text-sm text-muted-foreground">{t("silences.count", { count: silences.length })}</p>
        <Button size="sm" onClick={onAdd} data-testid="add-silence-btn">
          <Plus className="h-4 w-4 mr-1" />
          {t("silences.create")}
        </Button>
      </div>
      {silences.length === 0 ? (
        <p className="text-center text-muted-foreground py-8 text-sm">{t("silences.empty")}</p>
      ) : (
        <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("silences.table.monitor")}</TableHead>
                <TableHead className="hidden md:table-cell">{t("silences.table.reason")}</TableHead>
                <TableHead className="hidden md:table-cell">{t("silences.table.startsAt")}</TableHead>
                <TableHead className="hidden md:table-cell">{t("silences.table.endsAt")}</TableHead>
                <TableHead>{t("silences.table.status")}</TableHead>
                <TableHead className="w-16" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {silences.map((sil) => (
                <TableRow key={sil.id} data-testid={`silence-row-${sil.id}`}>
                  <TableCell>{sil.monitorName ?? t("silences.global")}</TableCell>
                  <TableCell className="hidden md:table-cell">{sil.reason}</TableCell>
                  <TableCell className="hidden md:table-cell">{new Date(sil.startsAt).toLocaleString("zh-CN")}</TableCell>
                  <TableCell className="hidden md:table-cell">{new Date(sil.endsAt).toLocaleString("zh-CN")}</TableCell>
                  <TableCell>{silenceStatusBadge(sil.status)}</TableCell>
                  <TableCell>
                    {sil.status !== "expired" && (
                      <Button
                        variant="ghost"
                        size="icon"
                        onClick={() => onDelete(sil.id)}
                        aria-label={`${t("silences.table.status")} ${sil.id}`}
                        data-testid={`delete-silence-btn-${sil.id}`}
                      >
                        <Trash2 className="h-4 w-4 text-muted-foreground" />
                      </Button>
                    )}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
        </Table>
      )}
    </div>
  )
}

// ─── Add Silence Dialog ───────────────────────────────────────────────────────

interface AddSilenceDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onCreated: (silence: AlertSilence) => void
}

interface MonitorOption {
  id: string
  name: string
}

function AddSilenceDialog({ open, onOpenChange, onCreated }: AddSilenceDialogProps) {
  const t = useTranslations("alerts")
  const [monitors, setMonitors] = useState<MonitorOption[]>([])
  const [monitorsLoading, setMonitorsLoading] = useState(true)
  const [monitorId, setMonitorId] = useState<string>("__global__")
  const [reason, setReason] = useState("")
  const [startsAt, setStartsAt] = useState("")
  const [endsAt, setEndsAt] = useState("")
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!open) return
    let cancelled = false
    setMonitorsLoading(true)
    apiRequest<{ data: { items: MonitorOption[] } }>("/v1/monitors")
      .then((res) => { if (!cancelled) setMonitors(res.data.items ?? []) })
      .catch(() => { if (!cancelled) setMonitors([]) })
      .finally(() => { if (!cancelled) setMonitorsLoading(false) })
    return () => { cancelled = true }
  }, [open])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!reason.trim() || !startsAt || !endsAt) return
    setSubmitting(true)
    setError(null)
    try {
      const body: Record<string, unknown> = {
        reason: reason.trim(),
        starts_at: new Date(startsAt).toISOString(),
        ends_at: new Date(endsAt).toISOString(),
      }
      if (monitorId !== "__global__") body.monitor_id = monitorId
      const res = await apiRequest<{ data: { silence: AlertSilence } }>("/v1/alert-silences", {
        method: "POST",
        body: JSON.stringify(body),
      })
      onCreated(res.data.silence)
      onOpenChange(false)
      setReason("")
      setStartsAt("")
      setEndsAt("")
      setMonitorId("__global__")
    } catch (err) {
      setError(err instanceof Error ? err.message : t("silences.addDialog.createFailed"))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent data-testid="add-silence-dialog">
        <DialogHeader>
          <DialogTitle>{t("silences.addDialog.title")}</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="space-y-4 mt-2">
          {error && (
            <Alert variant="destructive">
              <AlertCircle className="h-4 w-4" />
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          )}
          <div className="space-y-2">
            <Label htmlFor="silence-monitor">{t("silences.addDialog.monitor")}</Label>
            <Select value={monitorId} onValueChange={setMonitorId} disabled={monitorsLoading}>
              <SelectTrigger id="silence-monitor">
                <SelectValue placeholder={monitorsLoading ? t("policies.form.loading") : t("silences.addDialog.globalSilence")} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="__global__">{t("silences.addDialog.globalSilence")}</SelectItem>
                {monitors.map((m) => (
                  <SelectItem key={m.id} value={m.id}>{m.name}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-2">
            <Label htmlFor="silence-reason">{t("silences.addDialog.reason")}</Label>
            <Textarea
              id="silence-reason"
              placeholder={t("silences.addDialog.reasonPlaceholder")}
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              rows={2}
              required
            />
          </div>
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="silence-starts">{t("silences.addDialog.startsAt")}</Label>
              <Input
                id="silence-starts"
                type="datetime-local"
                value={startsAt}
                onChange={(e) => setStartsAt(e.target.value)}
                required
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="silence-ends">{t("silences.addDialog.endsAt")}</Label>
              <Input
                id="silence-ends"
                type="datetime-local"
                value={endsAt}
                onChange={(e) => setEndsAt(e.target.value)}
                required
              />
            </div>
          </div>
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
              disabled={submitting}
            >
              {t("confirm.cancel")}
            </Button>
            <Button
              type="submit"
              disabled={submitting || !reason.trim() || !startsAt || !endsAt}
            >
              {submitting ? t("silences.addDialog.submitting") : t("silences.addDialog.submit")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

// ─── API response shapes ──────────────────────────────────────────────────────

interface ChannelsResponse {
  data: { items: AlertChannel[] }
}
interface PoliciesResponse {
  data: { items: AlertPolicy[] }
}

// ─── Skeleton loaders ─────────────────────────────────────────────────────────

function ChannelsSkeleton() {
  return (
    <div className="space-y-4" data-testid="channels-skeleton">
      <div className="flex justify-between">
        <Skeleton className="h-5 w-24 rounded" />
        <Skeleton className="h-8 w-24 rounded" />
      </div>
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {Array.from({ length: 3 }).map((_, i) => (
          <Skeleton key={i} className="h-40 w-full rounded-lg" />
        ))}
      </div>
    </div>
  )
}

function PoliciesSkeleton() {
  return (
    <div className="space-y-4" data-testid="policies-skeleton">
      <div className="flex justify-between">
        <Skeleton className="h-5 w-24 rounded" />
        <Skeleton className="h-8 w-24 rounded" />
      </div>
      <Card>
        <div className="p-4 space-y-3">
          {Array.from({ length: 3 }).map((_, i) => (
            <Skeleton key={i} className="h-8 w-full rounded" />
          ))}
        </div>
      </Card>
    </div>
  )
}

// ─── Main AlertsClient ────────────────────────────────────────────────────────

export function AlertsClient() {
  const t = useTranslations("alerts")
  // URL-synced tab state
  const searchParams = useSearchParams()
  const router = useRouter()
  const pathname = usePathname()
  const initialTab = searchParams.get("tab") ?? "events"
  const [activeTab, setActiveTab] = useState(initialTab)

  // Read ?monitor= param to pre-fill EventsTab monitor filter
  const initialMonitorId = searchParams.get("monitor") ?? ""

  // Per-tab data state
  const [channels, setChannels] = useState<AlertChannel[]>([])
  const [policies, setPolicies] = useState<AlertPolicy[]>([])
  const [silences, setSilences] = useState<AlertSilence[]>([])

  // Track which channel IDs have an in-flight test request
  const [testingIds, setTestingIds] = useState<Set<string>>(new Set())

  // Per-tab loading & error state
  const [channelsLoading, setChannelsLoading] = useState(false)
  const [channelsError, setChannelsError] = useState<string | null>(null)
  const [policiesLoading, setPoliciesLoading] = useState(false)
  const [policiesError, setPoliciesError] = useState<string | null>(null)
  const [silencesLoading, setSilencesLoading] = useState(false)
  const [silencesError, setSilencesError] = useState<string | null>(null)

  // Track which tabs have been fetched
  const fetchedRef = useRef<Record<string, boolean>>({ channels: false, policies: false, silences: false })

  // Silence dialog state
  const [showAddSilence, setShowAddSilence] = useState(false)

  // Confirm dialog state
  const [confirm, setConfirm] = useState<{
    open: boolean
    title: string
    description: string
    onConfirm: () => void
  }>({ open: false, title: "", description: "", onConfirm: () => {} })

  // Sheet state
  const [sheet, setSheet] = useState<{
    open: boolean
    mode: "add-channel" | "add-policy" | "edit-policy"
    policy?: AlertPolicy
  }>({ open: false, mode: "add-channel" })

  // ── Data fetchers ──

  const fetchChannels = useCallback(async () => {
    if (fetchedRef.current.channels) return
    fetchedRef.current.channels = true
    setChannelsLoading(true)
    setChannelsError(null)
    try {
      const res = await apiRequest<ChannelsResponse>("/v1/alert-channels")
      setChannels(res.data.channels ?? [])
    } catch (err) {
      setChannelsError(err instanceof Error ? err.message : t("events.loadFailed"))
    } finally {
      setChannelsLoading(false)
    }
  }, [t])

  const fetchPolicies = useCallback(async () => {
    if (fetchedRef.current.policies) return
    fetchedRef.current.policies = true
    setPoliciesLoading(true)
    setPoliciesError(null)
    try {
      const res = await apiRequest<PoliciesResponse>("/v1/alert-policies")
      setPolicies(res.data.policies ?? [])
    } catch (err) {
      setPoliciesError(err instanceof Error ? err.message : t("events.loadFailed"))
    } finally {
      setPoliciesLoading(false)
    }
  }, [t])

  const fetchSilences = useCallback(async () => {
    if (fetchedRef.current.silences) return
    fetchedRef.current.silences = true
    setSilencesLoading(true)
    setSilencesError(null)
    try {
      const res = await apiRequest<{ data: { silences: AlertSilence[] } }>("/v1/alert-silences")
      setSilences(res.data.silences ?? [])
    } catch (err) {
      setSilencesError(err instanceof Error ? err.message : t("events.loadFailed"))
    } finally {
      setSilencesLoading(false)
    }
  }, [t])

  // ── Tab change handler ──

  const handleTabChange = (value: string) => {
    setActiveTab(value)
    const params = new URLSearchParams(searchParams.toString())
    params.set("tab", value)
    // Remove transient ?monitor= param on tab switch so it doesn't persist
    params.delete("monitor")
    router.replace(`${pathname}?${params.toString()}`, { scroll: false })
    if (value === "channels") void fetchChannels()
    if (value === "policies") void fetchPolicies()
    if (value === "silences") void fetchSilences()
  }

  // ── Fetch data for the initial tab if it isn't "events" ──

  useEffect(() => {
    if (initialTab === "channels") void fetchChannels()
    else if (initialTab === "policies") void fetchPolicies()
    else if (initialTab === "silences") void fetchSilences()
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // ── Event handlers ──

  const handleTestChannel = async (id: string) => {
    const ch = channels.find((c) => c.id === id)
    setTestingIds((prev) => new Set(prev).add(id))
    try {
      await apiRequest(`/v1/alert-channels/${id}/test`, { method: "POST" })
      // Mark channel as verified locally
      setChannels((prev) =>
        prev.map((c) => (c.id === id ? { ...c, verified: true } : c))
      )
      toast.success(t("channels.testSent"), {
        description: ch?.name ? t("channels.testSentDesc", { name: ch.name }) : undefined,
      })
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("channels.testFailed"))
    } finally {
      setTestingIds((prev) => {
        const next = new Set(prev)
        next.delete(id)
        return next
      })
    }
  }

  const handleDeleteChannel = (id: string) => {
    const ch = channels.find((c) => c.id === id)
    setConfirm({
      open: true,
      title: t("channels.delete.title"),
      description: t("channels.delete.desc", { name: ch?.name ?? "" }),
      onConfirm: async () => {
        try {
          await apiRequest(`/v1/alert-channels/${id}`, { method: "DELETE" })
          setChannels((prev) => prev.filter((c) => c.id !== id))
          setConfirm((p) => ({ ...p, open: false }))
          toast(t("channels.delete.success", { name: ch?.name ?? "" }))
        } catch (err) {
          setConfirm((p) => ({ ...p, open: false }))
          toast(err instanceof Error ? err.message : t("channels.delete.failed"))
        }
      },
    })
  }

  const handleAddChannel = () =>
    setSheet({ open: true, mode: "add-channel" })

  const handleSaveChannel = async (partial: Omit<AlertChannel, "id" | "verified">) => {
    try {
      const res = await apiRequest<{ data: { channel: AlertChannel } }>("/v1/alert-channels", {
        method: "POST",
        body: JSON.stringify({
          name: partial.name,
          type: partial.type,
          config: { target: partial.config },
        }),
      })
      const newCh = res.data.channel
      setChannels((prev) => [...prev, newCh])
      setSheet((p) => ({ ...p, open: false }))
      toast(t("channels.add.success", { name: newCh.name }))
    } catch (err) {
      toast(err instanceof Error ? err.message : t("channels.add.failed"))
    }
  }

  const handleTogglePolicy = async (id: string) => {
    const pol = policies.find((p) => p.id === id)
    if (!pol) return
    const newEnabled = !pol.enabled
    // Optimistic update
    setPolicies((prev) =>
      prev.map((p) => (p.id === id ? { ...p, enabled: newEnabled } : p))
    )
    try {
      await apiRequest(`/v1/alert-policies/${id}`, {
        method: "PATCH",
        body: JSON.stringify({ enabled: newEnabled }),
      })
    } catch (err) {
      // Revert on failure
      setPolicies((prev) =>
        prev.map((p) => (p.id === id ? { ...p, enabled: pol.enabled } : p))
      )
      toast(err instanceof Error ? err.message : t("policies.save.failed"))
    }
  }

  const handleEditPolicy = (pol: AlertPolicy) =>
    setSheet({ open: true, mode: "edit-policy", policy: pol })

  const handleDeletePolicy = (id: string) => {
    const pol = policies.find((p) => p.id === id)
    setConfirm({
      open: true,
      title: t("policies.delete.title"),
      description: t("policies.delete.desc", { name: pol?.name ?? "" }),
      onConfirm: async () => {
        try {
          await apiRequest(`/v1/alert-policies/${id}`, { method: "DELETE" })
          setPolicies((prev) => prev.filter((p) => p.id !== id))
          setConfirm((p) => ({ ...p, open: false }))
          toast(t("policies.delete.success", { name: pol?.name ?? "" }))
        } catch (err) {
          setConfirm((p) => ({ ...p, open: false }))
          toast(err instanceof Error ? err.message : t("policies.delete.failed"))
        }
      },
    })
  }

  const handleAddPolicy = () =>
    setSheet({ open: true, mode: "add-policy" })

  const handleSavePolicy = async (partial: Omit<AlertPolicy, "id">) => {
    if (sheet.mode === "edit-policy" && sheet.policy) {
      try {
        const res = await apiRequest<{ data: { policy: AlertPolicy } }>(
          `/v1/alert-policies/${sheet.policy.id}`,
          {
            method: "PATCH",
            body: JSON.stringify(partial),
          }
        )
        const updated = res.data.policy
        setPolicies((prev) =>
          prev.map((p) => (p.id === sheet.policy!.id ? updated : p))
        )
        toast(t("policies.save.success"))
      } catch (err) {
        toast(err instanceof Error ? err.message : t("policies.save.failed"))
      }
    } else {
      try {
        const res = await apiRequest<{ data: { policy: AlertPolicy } }>("/v1/alert-policies", {
          method: "POST",
          body: JSON.stringify(partial),
        })
        const newPol = res.data.policy
        setPolicies((prev) => [...prev, newPol])
        toast(t("policies.save.createSuccess", { name: newPol.name }))
      } catch (err) {
        toast(err instanceof Error ? err.message : t("policies.save.createFailed"))
      }
    }
    setSheet((p) => ({ ...p, open: false }))
  }

  return (
    <div className="space-y-6">
      {/* Tabs — navigation + panels */}
      <Tabs value={activeTab} onValueChange={handleTabChange}>
        <TabsList className="w-full">
          <TabsTrigger value="events" className="flex-1" data-testid="tab-events">{t("tabs.events")}</TabsTrigger>
          <TabsTrigger value="channels" className="flex-1" data-testid="tab-channels">{t("tabs.channels")}</TabsTrigger>
          <TabsTrigger value="policies" className="flex-1" data-testid="tab-policies">{t("tabs.policies")}</TabsTrigger>
          <TabsTrigger value="silences" className="flex-1" data-testid="tab-silences">{t("tabs.silences")}</TabsTrigger>
        </TabsList>
        <TabsContent value="events">
          <EventsTab initialMonitorId={initialMonitorId} />
        </TabsContent>
        <TabsContent value="channels">
          {channelsLoading ? (
            <ChannelsSkeleton />
          ) : channelsError ? (
            <Alert variant="destructive" data-testid="channels-error">
              <AlertCircle className="h-4 w-4" />
              <AlertTitle>{t("events.loadFailed")}</AlertTitle>
              <AlertDescription>{channelsError}</AlertDescription>
            </Alert>
          ) : (
            <ChannelsTab
              channels={channels}
              testingIds={testingIds}
              onTest={handleTestChannel}
              onDelete={handleDeleteChannel}
              onAdd={handleAddChannel}
            />
          )}
        </TabsContent>
        <TabsContent value="policies">
          {policiesLoading ? (
            <PoliciesSkeleton />
          ) : policiesError ? (
            <Alert variant="destructive" data-testid="policies-error">
              <AlertCircle className="h-4 w-4" />
              <AlertTitle>{t("events.loadFailed")}</AlertTitle>
              <AlertDescription>{policiesError}</AlertDescription>
            </Alert>
          ) : (
            <PoliciesTab
              policies={policies}
              channels={channels}
              onToggle={handleTogglePolicy}
              onEdit={handleEditPolicy}
              onDelete={handleDeletePolicy}
              onAdd={handleAddPolicy}
            />
          )}
        </TabsContent>
        <TabsContent value="silences">
          {silencesLoading ? (
            <div className="space-y-4" data-testid="silences-skeleton">
              <div className="flex justify-between">
                <Skeleton className="h-5 w-24 rounded" />
                <Skeleton className="h-8 w-24 rounded" />
              </div>
              <Skeleton className="h-32 w-full rounded" />
            </div>
          ) : silencesError ? (
            <Alert variant="destructive" data-testid="silences-error">
              <AlertCircle className="h-4 w-4" />
              <AlertTitle>{t("events.loadFailed")}</AlertTitle>
              <AlertDescription>{silencesError}</AlertDescription>
            </Alert>
          ) : (
            <SilencesTab
              silences={silences}
              onDelete={async (id) => {
                try {
                  await apiRequest(`/v1/alert-silences/${id}`, { method: "DELETE" })
                  setSilences((prev) => prev.filter((s) => s.id !== id))
                  toast(t("silences.delete.success"))
                } catch (err) {
                  toast(t("silences.delete.failed", { msg: err instanceof Error ? err.message : "" }))
                }
              }}
              onAdd={() => setShowAddSilence(true)}
            />
          )}
        </TabsContent>
      </Tabs>

      {/* Confirm dialog */}
      <AlertDialog open={confirm.open} onOpenChange={(open) => !open && setConfirm((p) => ({ ...p, open: false }))}>
        <AlertDialogContent data-testid="confirm-dialog">
          <AlertDialogHeader>
            <AlertDialogTitle>{confirm.title}</AlertDialogTitle>
            <AlertDialogDescription>{confirm.description}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("confirm.cancel")}</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              onClick={confirm.onConfirm}
            >
              {t("confirm.delete")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* Side sheet */}
      <Sheet open={sheet.open} onOpenChange={(open) => !open && setSheet((p) => ({ ...p, open: false }))}>
        <SheetContent className="flex flex-col gap-0 p-0 overflow-hidden" data-testid="side-sheet">
          <SheetHeader className="shrink-0 border-b px-6 py-4">
            <SheetTitle>
              {sheet.mode === "add-channel"
                ? t("channels.addSheet")
                : sheet.mode === "edit-policy"
                ? t("policies.editSheet")
                : t("policies.addSheet")}
            </SheetTitle>
          </SheetHeader>
          <div className="flex-1 overflow-y-auto px-6 py-6">
            {sheet.mode === "add-channel" && (
              <AddChannelForm
                onSave={handleSaveChannel}
                onCancel={() => setSheet((p) => ({ ...p, open: false }))}
              />
            )}
            {(sheet.mode === "add-policy" || sheet.mode === "edit-policy") && (
              <PolicyForm
                channels={channels}
                initial={sheet.policy}
                onSave={handleSavePolicy}
                onCancel={() => setSheet((p) => ({ ...p, open: false }))}
              />
            )}
          </div>
          <SheetFooter className="shrink-0 border-t px-6 py-4 flex-row gap-3 mt-0">
            <Button
              type="submit"
              form={sheet.mode === "add-channel" ? "add-channel-form" : "policy-form"}
              className="flex-1"
            >
              {sheet.mode === "edit-policy" ? t("policies.sheet.saveChanges") : sheet.mode === "add-policy" ? t("policies.sheet.createPolicy") : t("policies.sheet.save")}
            </Button>
            <Button
              type="button"
              variant="outline"
              className="flex-1"
              onClick={() => setSheet((p) => ({ ...p, open: false }))}
            >
              {t("confirm.cancel")}
            </Button>
          </SheetFooter>
        </SheetContent>
      </Sheet>

      {/* Add silence dialog */}
      <AddSilenceDialog
        open={showAddSilence}
        onOpenChange={setShowAddSilence}
        onCreated={(silence) => {
          setSilences((prev) => [...prev, silence])
          toast(t("silences.addDialog.created"))
        }}
      />

    </div>
  )
}
