"use client"

import { useRef, useState, useEffect, useCallback } from "react"
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

function statusBadge(status: AlertEvent["status"]) {
  if (status === "firing")
    return <Badge variant="destructive">告警中</Badge>
  if (status === "resolved")
    return <Badge variant="success">已恢复</Badge>
  return <Badge variant="secondary">已确认</Badge>
}

function channelIcon(type: ChannelType) {
  if (type === "email") return <Mail className="h-4 w-4" aria-hidden />
  if (type === "wecom" || type === "feishu") return <MessageSquare className="h-4 w-4" aria-hidden />
  return <Bell className="h-4 w-4" aria-hidden />
}

function notifStatusBadge(status: AlertNotification["status"]) {
  if (status === "sent") return <Badge variant="success">成功</Badge>
  if (status === "failed") return <Badge variant="destructive">失败</Badge>
  return <Badge variant="secondary">待发</Badge>
}

// ─── Add Channel Form (inside Sheet) ─────────────────────────────────────────

interface AddChannelFormProps {
  onSave: (channel: Omit<AlertChannel, "id" | "verified">) => void
  onCancel: () => void
}

function AddChannelForm({ onSave, onCancel }: AddChannelFormProps) {
  const [type, setType] = useState<ChannelType>("email")
  const [name, setName] = useState("")
  const [config, setConfig] = useState("")

  const configLabel = type === "email" ? "收件邮箱" : "Webhook URL"
  const configPlaceholder =
    type === "email" ? "ops@example.com" : "https://your-webhook-url"

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim() || !config.trim()) return
    onSave({ name: name.trim(), type, config: config.trim() })
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-5">
      <div className="space-y-2">
        <Label htmlFor="channel-type">通道类型</Label>
        <Select value={type} onValueChange={(v) => setType(v as ChannelType)}>
          <SelectTrigger id="channel-type">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {CHANNEL_TYPES.map((t) => (
              <SelectItem key={t} value={t}>
                {CHANNEL_TYPE_LABELS[t]}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      <div className="space-y-2">
        <Label htmlFor="channel-name">通道名称</Label>
        <Input
          id="channel-name"
          placeholder="如：运维邮件组"
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

      <div className="flex gap-3 pt-2">
        <Button type="submit" className="flex-1">保存</Button>
        <Button type="button" variant="outline" className="flex-1" onClick={onCancel}>
          取消
        </Button>
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
    apiRequest<{ data: { monitors: MonitorOption[] } }>("/v1/monitors")
      .then((res) => {
        if (cancelled) return
        const list = res.data.monitors
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
    <form onSubmit={handleSubmit} className="space-y-5">
      <div className="space-y-2">
        <Label htmlFor="policy-name">策略名称</Label>
        <Input
          id="policy-name"
          placeholder="如：关键服务告警"
          value={name}
          onChange={(e) => setName(e.target.value)}
          required
        />
      </div>

      <div className="space-y-2">
        <Label htmlFor="policy-monitor">绑定监控</Label>
        <Select value={monitorName} onValueChange={setMonitorName} disabled={monitorsLoading}>
          <SelectTrigger id="policy-monitor" data-testid="policy-monitor-select">
            <SelectValue placeholder={monitorsLoading ? "加载中..." : "请选择监控"} />
          </SelectTrigger>
          <SelectContent>
            {monitorsLoading ? (
              <SelectItem value="__loading__" disabled>加载中...</SelectItem>
            ) : monitors.length === 0 ? (
              <SelectItem value="__empty__" disabled>暂无监控</SelectItem>
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
        <Label>告警通道</Label>
        <div className="space-y-2 rounded-md border p-3">
          {channels.length === 0 && (
            <p className="text-sm text-muted-foreground">暂无通道，请先添加</p>
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
        </div>
      </div>

      <div className="space-y-2">
        <Label>延迟告警（{delay} 分钟）</Label>
        <Slider
          min={0}
          max={60}
          step={1}
          value={[delay]}
          onValueChange={([v]) => setDelay(v!)}
          aria-label="延迟告警分钟数"
        />
        <div className="flex justify-between text-xs text-muted-foreground">
          <span>立即</span>
          <span>60 分钟</span>
        </div>
      </div>

      <div className="space-y-2">
        <Label>静音时段</Label>
        <div className="flex items-center gap-3">
          <Input
            type="time"
            value={muteFrom}
            onChange={(e) => setMuteFrom(e.target.value)}
            aria-label="静音开始时间"
            className="flex-1"
          />
          <span className="text-sm text-muted-foreground">至</span>
          <Input
            type="time"
            value={muteTo}
            onChange={(e) => setMuteTo(e.target.value)}
            aria-label="静音结束时间"
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
        <Label htmlFor="policy-enabled" className="cursor-pointer text-sm">启用策略</Label>
      </div>

      <div className="flex gap-3 pt-2">
        <Button type="submit" className="flex-1">
          {initial ? "保存更改" : "创建策略"}
        </Button>
        <Button type="button" variant="outline" className="flex-1" onClick={onCancel}>
          取消
        </Button>
      </div>
    </form>
  )
}

// ─── Events Tab ───────────────────────────────────────────────────────────────

interface EventsTabProps {
  events: AlertEvent[]
  onAcknowledge: (id: string) => void
}

function EventsTab({ events, onAcknowledge }: EventsTabProps) {
  const firingCount = events.filter((e) => e.status === "firing").length

  return (
    <div className="space-y-4">
      {firingCount > 0 && (
        <Alert variant="destructive" data-testid="firing-alert">
          <AlertCircle className="h-4 w-4" />
          <AlertTitle>活跃告警</AlertTitle>
          <AlertDescription>
            当前有 <strong>{firingCount}</strong> 个告警正在触发，请及时处理。
          </AlertDescription>
        </Alert>
      )}

      <Card className="overflow-x-auto">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>监控名</TableHead>
              <TableHead>状态</TableHead>
              <TableHead className="hidden md:table-cell">开始时间</TableHead>
              <TableHead className="hidden md:table-cell">持续时长</TableHead>
              <TableHead>操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {events.map((evt) => (
              <TableRow key={evt.id} data-testid={`event-row-${evt.id}`}>
                <TableCell className="font-medium">{evt.monitorName}</TableCell>
                <TableCell>{statusBadge(evt.status)}</TableCell>
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
                      onClick={() => onAcknowledge(evt.id)}
                      data-testid={`ack-btn-${evt.id}`}
                    >
                      <CheckCheck className="mr-1 h-3 w-3" />
                      Acknowledge
                    </Button>
                  )}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </Card>
    </div>
  )
}

// ─── Channels Tab ─────────────────────────────────────────────────────────────

interface ChannelsTabProps {
  channels: AlertChannel[]
  onTest: (id: string) => void
  onDelete: (id: string) => void
  onAdd: () => void
}

interface NotificationsResponse {
  data: { notifications: AlertNotification[] }
}

function ChannelDeliveryHistory({ channelId }: { channelId: string }) {
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
        查看交付记录（{notifications.length} 条）
        {open ? <ChevronUp className="h-3 w-3" /> : <ChevronDown className="h-3 w-3" />}
      </Button>
      {open && (
        <div data-testid={`delivery-history-content-${channelId}`} className="px-4 pb-3">
          {notifLoading ? (
            <p className="py-2 text-center text-xs text-muted-foreground">加载中...</p>
          ) : notifications.length === 0 ? (
            <p className="py-2 text-center text-xs text-muted-foreground">暂无交付记录</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="text-xs">状态</TableHead>
                  <TableHead className="text-xs">告警事件</TableHead>
                  <TableHead className="text-xs">发送时间</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {notifications.slice(0, 10).map((n) => (
                  <TableRow key={n.id} data-testid={`notif-row-${n.id}`}>
                    <TableCell className="py-1">{notifStatusBadge(n.status)}</TableCell>
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

function ChannelsTab({ channels, onTest, onDelete, onAdd }: ChannelsTabProps) {
  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <p className="text-sm text-muted-foreground">共 {channels.length} 个通道</p>
        <Button size="sm" onClick={onAdd} data-testid="add-channel-btn">
          <Plus className="mr-1 h-4 w-4" />
          添加通道
        </Button>
      </div>

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {channels.map((ch) => (
          <Card key={ch.id} data-testid={`channel-card-${ch.id}`}>
            <CardHeader className="pb-3">
              <CardTitle className="flex items-center gap-2 text-sm font-semibold">
                {channelIcon(ch.type)}
                <span className="flex-1 truncate">{ch.name}</span>
                <Badge variant={ch.verified ? "success" : "warning"} className="shrink-0">
                  {ch.verified ? "已验证" : "未验证"}
                </Badge>
              </CardTitle>
              <p className="text-xs text-muted-foreground">
                {CHANNEL_TYPE_LABELS[ch.type]}
              </p>
            </CardHeader>
            <CardContent className="pb-2">
              <p className="truncate font-mono text-xs text-muted-foreground">
                {truncateConfig(ch.config)}
              </p>
            </CardContent>
            <CardFooter className="flex gap-2 pt-0">
              <Button
                variant="outline"
                size="sm"
                className="flex-1"
                onClick={() => onTest(ch.id)}
                data-testid={`test-channel-btn-${ch.id}`}
              >
                测试发送
              </Button>
              <Button
                variant="outline"
                size="sm"
                className="text-destructive hover:bg-destructive/10"
                onClick={() => onDelete(ch.id)}
                data-testid={`delete-channel-btn-${ch.id}`}
                aria-label={`删除通道 ${ch.name}`}
              >
                <Trash2 className="h-4 w-4" />
              </Button>
            </CardFooter>
            <ChannelDeliveryHistory channelId={ch.id} />
          </Card>
        ))}
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
  const channelName = (id: string) =>
    channels.find((c) => c.id === id)?.name ?? id

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <p className="text-sm text-muted-foreground">共 {policies.length} 条策略</p>
        <Button size="sm" onClick={onAdd} data-testid="add-policy-btn">
          <Plus className="mr-1 h-4 w-4" />
          新建策略
        </Button>
      </div>

      <Card className="overflow-x-auto">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>策略名</TableHead>
              <TableHead className="hidden md:table-cell">绑定监控</TableHead>
              <TableHead className="hidden md:table-cell">通道</TableHead>
              <TableHead className="hidden md:table-cell">延迟</TableHead>
              <TableHead className="hidden md:table-cell">静音时段</TableHead>
              <TableHead>状态</TableHead>
              <TableHead>操作</TableHead>
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
                  {pol.delayMinutes === 0 ? "立即" : `${pol.delayMinutes} 分钟`}
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
                      aria-label={`策略 ${pol.name} ${pol.enabled ? "已启用" : "已关闭"}`}
                      data-testid={`policy-toggle-${pol.id}`}
                    />
                    <span className="text-xs text-muted-foreground">
                      {pol.enabled ? "启用" : "关闭"}
                    </span>
                  </div>
                </TableCell>
                <TableCell>
                  <div className="flex items-center gap-1">
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={() => onEdit(pol)}
                      aria-label={`编辑策略 ${pol.name}`}
                      data-testid={`edit-policy-btn-${pol.id}`}
                    >
                      <Pencil className="h-4 w-4" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="text-destructive hover:bg-destructive/10"
                      onClick={() => onDelete(pol.id)}
                      aria-label={`删除策略 ${pol.name}`}
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
      </Card>
    </div>
  )
}

// ─── Silences Tab ────────────────────────────────────────────────────────────

interface SilencesTabProps {
  silences: AlertSilence[]
  onDelete: (id: string) => void
  onAdd: () => void
}

function silenceStatusBadge(status: AlertSilence["status"]) {
  if (status === "active") return <Badge variant="destructive">生效中</Badge>
  if (status === "upcoming") return <Badge variant="secondary">即将生效</Badge>
  return <Badge variant="outline">已过期</Badge>
}

function SilencesTab({ silences, onDelete, onAdd }: SilencesTabProps) {
  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <p className="text-sm text-muted-foreground">共 {silences.length} 条静默规则</p>
        <Button size="sm" onClick={onAdd} data-testid="add-silence-btn">
          <Plus className="h-4 w-4 mr-1" />
          添加静默
        </Button>
      </div>
      {silences.length === 0 ? (
        <p className="text-center text-muted-foreground py-8 text-sm">暂无静默规则</p>
      ) : (
        <div className="overflow-x-auto rounded-md border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>监控</TableHead>
                <TableHead className="hidden md:table-cell">原因</TableHead>
                <TableHead className="hidden md:table-cell">开始时间</TableHead>
                <TableHead className="hidden md:table-cell">结束时间</TableHead>
                <TableHead>状态</TableHead>
                <TableHead className="w-16" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {silences.map((sil) => (
                <TableRow key={sil.id} data-testid={`silence-row-${sil.id}`}>
                  <TableCell>{sil.monitorName ?? "全局"}</TableCell>
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
                        aria-label={`提前结束静默 ${sil.id}`}
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
        </div>
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
    apiRequest<{ data: { monitors: MonitorOption[] } }>("/v1/monitors")
      .then((res) => { if (!cancelled) setMonitors(res.data.monitors) })
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
      setError(err instanceof Error ? err.message : "创建失败，请重试")
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent data-testid="add-silence-dialog">
        <DialogHeader>
          <DialogTitle>添加静默规则</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="space-y-4 mt-2">
          {error && (
            <Alert variant="destructive">
              <AlertCircle className="h-4 w-4" />
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          )}
          <div className="space-y-2">
            <Label htmlFor="silence-monitor">绑定监控（可选）</Label>
            <Select value={monitorId} onValueChange={setMonitorId} disabled={monitorsLoading}>
              <SelectTrigger id="silence-monitor">
                <SelectValue placeholder={monitorsLoading ? "加载中..." : "全局静默"} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="__global__">全局静默</SelectItem>
                {monitors.map((m) => (
                  <SelectItem key={m.id} value={m.id}>{m.name}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-2">
            <Label htmlFor="silence-reason">原因</Label>
            <Textarea
              id="silence-reason"
              placeholder="如：计划维护窗口"
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              rows={2}
              required
            />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-2">
              <Label htmlFor="silence-starts">开始时间</Label>
              <Input
                id="silence-starts"
                type="datetime-local"
                value={startsAt}
                onChange={(e) => setStartsAt(e.target.value)}
                required
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="silence-ends">结束时间</Label>
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
              取消
            </Button>
            <Button
              type="submit"
              disabled={submitting || !reason.trim() || !startsAt || !endsAt}
            >
              {submitting ? "创建中..." : "创建静默"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

// ─── API response shapes ──────────────────────────────────────────────────────

interface EventsResponse {
  data: { events: AlertEvent[] }
}
interface ChannelsResponse {
  data: { channels: AlertChannel[] }
}
interface PoliciesResponse {
  data: { policies: AlertPolicy[] }
}

// ─── Skeleton loaders ─────────────────────────────────────────────────────────

function EventsSkeleton() {
  return (
    <div className="space-y-4" data-testid="events-skeleton">
      <Skeleton className="h-14 w-full rounded-md" />
      <Card>
        <div className="p-4 space-y-3">
          {Array.from({ length: 5 }).map((_, i) => (
            <Skeleton key={i} className="h-8 w-full rounded" />
          ))}
        </div>
      </Card>
    </div>
  )
}

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
  // Per-tab data state
  const [events, setEvents] = useState<AlertEvent[]>([])
  const [channels, setChannels] = useState<AlertChannel[]>([])
  const [policies, setPolicies] = useState<AlertPolicy[]>([])
  const [silences, setSilences] = useState<AlertSilence[]>([])

  // Per-tab loading & error state
  const [eventsLoading, setEventsLoading] = useState(true)
  const [eventsError, setEventsError] = useState<string | null>(null)
  const [channelsLoading, setChannelsLoading] = useState(false)
  const [channelsError, setChannelsError] = useState<string | null>(null)
  const [policiesLoading, setPoliciesLoading] = useState(false)
  const [policiesError, setPoliciesError] = useState<string | null>(null)
  const [silencesLoading, setSilencesLoading] = useState(false)
  const [silencesError, setSilencesError] = useState<string | null>(null)

  // Track which tabs have been fetched
  const fetchedRef = useRef<Record<string, boolean>>({ events: false, channels: false, policies: false, silences: false })

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

  const fetchEvents = useCallback(async () => {
    if (fetchedRef.current.events) return
    fetchedRef.current.events = true
    setEventsLoading(true)
    setEventsError(null)
    try {
      const res = await apiRequest<EventsResponse>("/v1/alert-events")
      setEvents(res.data.events)
    } catch (err) {
      setEventsError(err instanceof Error ? err.message : "加载失败")
    } finally {
      setEventsLoading(false)
    }
  }, [])

  const fetchChannels = useCallback(async () => {
    if (fetchedRef.current.channels) return
    fetchedRef.current.channels = true
    setChannelsLoading(true)
    setChannelsError(null)
    try {
      const res = await apiRequest<ChannelsResponse>("/v1/alert-channels")
      setChannels(res.data.channels)
    } catch (err) {
      setChannelsError(err instanceof Error ? err.message : "加载失败")
    } finally {
      setChannelsLoading(false)
    }
  }, [])

  const fetchPolicies = useCallback(async () => {
    if (fetchedRef.current.policies) return
    fetchedRef.current.policies = true
    setPoliciesLoading(true)
    setPoliciesError(null)
    try {
      const res = await apiRequest<PoliciesResponse>("/v1/alert-policies")
      setPolicies(res.data.policies)
    } catch (err) {
      setPoliciesError(err instanceof Error ? err.message : "加载失败")
    } finally {
      setPoliciesLoading(false)
    }
  }, [])

  const fetchSilences = useCallback(async () => {
    if (fetchedRef.current.silences) return
    fetchedRef.current.silences = true
    setSilencesLoading(true)
    setSilencesError(null)
    try {
      const res = await apiRequest<{ data: { silences: AlertSilence[] } }>("/v1/alert-silences")
      setSilences(res.data.silences ?? [])
    } catch (err) {
      setSilencesError(err instanceof Error ? err.message : "加载失败")
    } finally {
      setSilencesLoading(false)
    }
  }, [])

  // Fetch events on mount (default tab)
  useEffect(() => {
    void fetchEvents()
  }, [fetchEvents])

  // ── Tab change handler ──

  const handleTabChange = (value: string) => {
    if (value === "channels") void fetchChannels()
    if (value === "policies") void fetchPolicies()
    if (value === "silences") void fetchSilences()
  }

  // ── Event handlers ──

  const handleAcknowledge = async (id: string) => {
    try {
      await apiRequest(`/v1/alert-events/${id}/ack`, { method: "POST" })
      setEvents((prev) =>
        prev.map((e) =>
          e.id === id
            ? { ...e, status: "acknowledged", acknowledgedAt: new Date().toISOString() }
            : e
        )
      )
      toast("告警已确认")
    } catch (err) {
      toast(err instanceof Error ? err.message : "确认失败，请重试")
    }
  }

  const handleTestChannel = async (id: string) => {
    const ch = channels.find((c) => c.id === id)
    try {
      await apiRequest(`/v1/alert-channels/${id}/test`, { method: "POST" })
      toast(`测试消息已发送至 ${ch?.name ?? id}`)
    } catch (err) {
      toast(`发送失败: ${err instanceof Error ? err.message : "请重试"}`)
    }
  }

  const handleDeleteChannel = (id: string) => {
    const ch = channels.find((c) => c.id === id)
    setConfirm({
      open: true,
      title: "删除通道",
      description: `确认删除通道 "${ch?.name}"？此操作不可撤销。`,
      onConfirm: async () => {
        try {
          await apiRequest(`/v1/alert-channels/${id}`, { method: "DELETE" })
          setChannels((prev) => prev.filter((c) => c.id !== id))
          setConfirm((p) => ({ ...p, open: false }))
          toast(`通道 "${ch?.name}" 已删除`)
        } catch (err) {
          setConfirm((p) => ({ ...p, open: false }))
          toast(err instanceof Error ? err.message : "删除失败，请重试")
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
      toast(`通道 "${newCh.name}" 已添加`)
    } catch (err) {
      toast(err instanceof Error ? err.message : "创建失败，请重试")
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
      toast(err instanceof Error ? err.message : "更新失败，请重试")
    }
  }

  const handleEditPolicy = (pol: AlertPolicy) =>
    setSheet({ open: true, mode: "edit-policy", policy: pol })

  const handleDeletePolicy = (id: string) => {
    const pol = policies.find((p) => p.id === id)
    setConfirm({
      open: true,
      title: "删除策略",
      description: `确认删除策略 "${pol?.name}"？此操作不可撤销。`,
      onConfirm: async () => {
        try {
          await apiRequest(`/v1/alert-policies/${id}`, { method: "DELETE" })
          setPolicies((prev) => prev.filter((p) => p.id !== id))
          setConfirm((p) => ({ ...p, open: false }))
          toast(`策略 "${pol?.name}" 已删除`)
        } catch (err) {
          setConfirm((p) => ({ ...p, open: false }))
          toast(err instanceof Error ? err.message : "删除失败，请重试")
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
        toast("策略已更新")
      } catch (err) {
        toast(err instanceof Error ? err.message : "更新失败，请重试")
      }
    } else {
      try {
        const res = await apiRequest<{ data: { policy: AlertPolicy } }>("/v1/alert-policies", {
          method: "POST",
          body: JSON.stringify(partial),
        })
        const newPol = res.data.policy
        setPolicies((prev) => [...prev, newPol])
        toast(`策略 "${newPol.name}" 已创建`)
      } catch (err) {
        toast(err instanceof Error ? err.message : "创建失败，请重试")
      }
    }
    setSheet((p) => ({ ...p, open: false }))
  }

  return (
    <div className="space-y-6">
      {/* Tabs — navigation + panels */}
      <Tabs defaultValue="events" onValueChange={handleTabChange}>
        <TabsList className="w-full">
          <TabsTrigger value="events" className="flex-1" data-testid="tab-events">事件历史</TabsTrigger>
          <TabsTrigger value="channels" className="flex-1" data-testid="tab-channels">告警通道</TabsTrigger>
          <TabsTrigger value="policies" className="flex-1" data-testid="tab-policies">告警策略</TabsTrigger>
          <TabsTrigger value="silences" className="flex-1" data-testid="tab-silences">静默规则</TabsTrigger>
        </TabsList>
        <TabsContent value="events">
          {eventsLoading ? (
            <EventsSkeleton />
          ) : eventsError ? (
            <Alert variant="destructive" data-testid="events-error">
              <AlertCircle className="h-4 w-4" />
              <AlertTitle>加载失败</AlertTitle>
              <AlertDescription>{eventsError}</AlertDescription>
            </Alert>
          ) : (
            <EventsTab events={events} onAcknowledge={handleAcknowledge} />
          )}
        </TabsContent>
        <TabsContent value="channels">
          {channelsLoading ? (
            <ChannelsSkeleton />
          ) : channelsError ? (
            <Alert variant="destructive" data-testid="channels-error">
              <AlertCircle className="h-4 w-4" />
              <AlertTitle>加载失败</AlertTitle>
              <AlertDescription>{channelsError}</AlertDescription>
            </Alert>
          ) : (
            <ChannelsTab
              channels={channels}
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
              <AlertTitle>加载失败</AlertTitle>
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
              <AlertTitle>加载失败</AlertTitle>
              <AlertDescription>{silencesError}</AlertDescription>
            </Alert>
          ) : (
            <SilencesTab
              silences={silences}
              onDelete={async (id) => {
                try {
                  await apiRequest(`/v1/alert-silences/${id}`, { method: "DELETE" })
                  setSilences((prev) => prev.filter((s) => s.id !== id))
                  toast("静默规则已提前结束")
                } catch (err) {
                  toast(`删除失败: ${err instanceof Error ? err.message : "请重试"}`)
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
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              onClick={confirm.onConfirm}
            >
              删除
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* Side sheet */}
      <Sheet open={sheet.open} onOpenChange={(open) => !open && setSheet((p) => ({ ...p, open: false }))}>
        <SheetContent data-testid="side-sheet">
          <SheetHeader>
            <SheetTitle>
              {sheet.mode === "add-channel"
                ? "添加告警通道"
                : sheet.mode === "edit-policy"
                ? "编辑告警策略"
                : "新建告警策略"}
            </SheetTitle>
          </SheetHeader>
          <div className="mt-4 overflow-y-auto">
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
        </SheetContent>
      </Sheet>

      {/* Add silence dialog */}
      <AddSilenceDialog
        open={showAddSilence}
        onOpenChange={setShowAddSilence}
        onCreated={(silence) => {
          setSilences((prev) => [...prev, silence])
          toast("静默规则已创建")
        }}
      />

    </div>
  )
}
