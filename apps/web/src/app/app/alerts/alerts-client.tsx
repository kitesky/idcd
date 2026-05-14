"use client"

import { useRef, useState } from "react"
import {
  Bell,
  Mail,
  MessageSquare,
  CheckCheck,
  Plus,
  Trash2,
  Pencil,
  X,
  AlertCircle,
} from "lucide-react"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardHeader, CardTitle, CardFooter } from "@/components/ui/card"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import {
  type AlertEvent,
  type AlertChannel,
  type AlertPolicy,
  type ChannelType,
  MOCK_ALERT_EVENTS,
  MOCK_ALERT_CHANNELS,
  MOCK_ALERT_POLICIES,
  MOCK_MONITOR_NAMES,
  CHANNEL_TYPE_LABELS,
  CHANNEL_TYPES,
  formatDuration,
  truncateConfig,
} from "./mock-data"

// ─── Types ───────────────────────────────────────────────────────────────────

type Tab = "events" | "channels" | "policies"

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

// ─── Confirmation Dialog (inline, no Radix Dialog needed) ─────────────────

interface ConfirmDialogProps {
  open: boolean
  title: string
  description: string
  onConfirm: () => void
  onCancel: () => void
}

function ConfirmDialog({ open, title, description, onConfirm, onCancel }: ConfirmDialogProps) {
  if (!open) return null
  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60"
      role="dialog"
      aria-modal="true"
      aria-labelledby="confirm-title"
      data-testid="confirm-dialog"
    >
      <div className="w-full max-w-sm rounded-lg border bg-background p-6 shadow-lg">
        <h2 id="confirm-title" className="text-lg font-semibold">{title}</h2>
        <p className="mt-2 text-sm text-muted-foreground">{description}</p>
        <div className="mt-6 flex justify-end gap-3">
          <Button variant="outline" onClick={onCancel}>取消</Button>
          <Button variant="destructive" onClick={onConfirm}>删除</Button>
        </div>
      </div>
    </div>
  )
}

// ─── Side Sheet (drawer-like panel) ──────────────────────────────────────────

interface SheetProps {
  open: boolean
  title: string
  onClose: () => void
  children: React.ReactNode
}

function Sheet({ open, title, onClose, children }: SheetProps) {
  if (!open) return null
  return (
    <div
      className="fixed inset-0 z-50 flex justify-end"
      role="dialog"
      aria-modal="true"
      aria-labelledby="sheet-title"
      data-testid="side-sheet"
    >
      <div className="absolute inset-0 bg-black/50" onClick={onClose} aria-hidden="true" />
      <div className="relative z-10 flex h-full w-full max-w-md flex-col border-l bg-background shadow-xl">
        <div className="flex items-center justify-between border-b px-6 py-4">
          <h2 id="sheet-title" className="text-lg font-semibold">{title}</h2>
          <Button variant="ghost" size="icon" onClick={onClose} aria-label="关闭">
            <X className="h-4 w-4" />
          </Button>
        </div>
        <div className="flex-1 overflow-y-auto px-6 py-4">{children}</div>
      </div>
    </div>
  )
}

// ─── Toast (simple in-page notification) ─────────────────────────────────────

interface ToastMsg {
  id: number
  message: string
}

function useToast() {
  const [toasts, setToasts] = useState<ToastMsg[]>([])
  const counterRef = useRef(0)

  const toast = (message: string) => {
    const id = ++counterRef.current
    setToasts((prev) => [...prev, { id, message }])
    setTimeout(() => {
      setToasts((prev) => prev.filter((t) => t.id !== id))
    }, 3000)
  }

  return { toasts, toast }
}

function ToastContainer({ toasts }: { toasts: ToastMsg[] }) {
  if (toasts.length === 0) return null
  return (
    <div className="fixed bottom-4 right-4 z-50 flex flex-col gap-2" role="status" aria-live="polite">
      {toasts.map((t) => (
        <div
          key={t.id}
          className="rounded-md border bg-background px-4 py-3 shadow-md text-sm"
          data-testid="toast"
        >
          {t.message}
        </div>
      ))}
    </div>
  )
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
  const [monitorName, setMonitorName] = useState(initial?.monitorName ?? MOCK_MONITOR_NAMES[0])
  const [selectedChannels, setSelectedChannels] = useState<string[]>(initial?.channelIds ?? [])
  const [delay, setDelay] = useState(initial?.delayMinutes ?? 5)
  const [muteFrom, setMuteFrom] = useState(initial?.muteFrom ?? "")
  const [muteTo, setMuteTo] = useState(initial?.muteTo ?? "")
  const [enabled, setEnabled] = useState(initial?.enabled ?? true)

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
        <Select value={monitorName} onValueChange={setMonitorName}>
          <SelectTrigger id="policy-monitor">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {MOCK_MONITOR_NAMES.map((n) => (
              <SelectItem key={n} value={n}>
                {n}
              </SelectItem>
            ))}
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
              <input
                type="checkbox"
                className="h-4 w-4 accent-primary"
                checked={selectedChannels.includes(ch.id)}
                onChange={() => toggleChannel(ch.id)}
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
        <Label htmlFor="policy-delay">延迟告警（{delay} 分钟）</Label>
        <input
          id="policy-delay"
          type="range"
          min={0}
          max={60}
          step={1}
          value={delay}
          onChange={(e) => setDelay(Number(e.target.value))}
          className="w-full accent-primary"
          aria-valuenow={delay}
          aria-valuemin={0}
          aria-valuemax={60}
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

      <div className="flex items-center gap-3">
        <label className="flex cursor-pointer items-center gap-2">
          <input
            type="checkbox"
            className="h-4 w-4 accent-primary"
            checked={enabled}
            onChange={(e) => setEnabled(e.target.checked)}
            aria-label="启用策略"
          />
          <span className="text-sm">启用策略</span>
        </label>
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

      <Card>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>监控名</TableHead>
              <TableHead>状态</TableHead>
              <TableHead>开始时间</TableHead>
              <TableHead>持续时长</TableHead>
              <TableHead>操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {events.map((evt) => (
              <TableRow key={evt.id} data-testid={`event-row-${evt.id}`}>
                <TableCell className="font-medium">{evt.monitorName}</TableCell>
                <TableCell>{statusBadge(evt.status)}</TableCell>
                <TableCell className="text-sm text-muted-foreground">
                  {new Date(evt.startedAt).toLocaleString("zh-CN", {
                    month: "numeric",
                    day: "numeric",
                    hour: "2-digit",
                    minute: "2-digit",
                  })}
                </TableCell>
                <TableCell className="text-sm text-muted-foreground tabular-nums">
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

      <Card>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>策略名</TableHead>
              <TableHead>绑定监控</TableHead>
              <TableHead>通道</TableHead>
              <TableHead>延迟</TableHead>
              <TableHead>静音时段</TableHead>
              <TableHead>状态</TableHead>
              <TableHead>操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {policies.map((pol) => (
              <TableRow key={pol.id} data-testid={`policy-row-${pol.id}`}>
                <TableCell className="font-medium">{pol.name}</TableCell>
                <TableCell className="text-sm text-muted-foreground">
                  {pol.monitorName}
                </TableCell>
                <TableCell>
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
                <TableCell className="text-sm text-muted-foreground tabular-nums">
                  {pol.delayMinutes === 0 ? "立即" : `${pol.delayMinutes} 分钟`}
                </TableCell>
                <TableCell className="text-sm text-muted-foreground tabular-nums">
                  {pol.muteFrom && pol.muteTo
                    ? `${pol.muteFrom}–${pol.muteTo}`
                    : "—"}
                </TableCell>
                <TableCell>
                  <label className="flex cursor-pointer items-center gap-2">
                    <input
                      type="checkbox"
                      role="switch"
                      className="h-4 w-4 accent-primary"
                      checked={pol.enabled}
                      onChange={() => onToggle(pol.id)}
                      aria-label={`策略 ${pol.name} ${pol.enabled ? "已启用" : "已关闭"}`}
                      data-testid={`policy-toggle-${pol.id}`}
                    />
                    <span className="text-xs text-muted-foreground">
                      {pol.enabled ? "启用" : "关闭"}
                    </span>
                  </label>
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

// ─── Main AlertsClient ────────────────────────────────────────────────────────

export function AlertsClient() {
  const [activeTab, setActiveTab] = useState<Tab>("events")
  const [events, setEvents] = useState<AlertEvent[]>(MOCK_ALERT_EVENTS)
  const [channels, setChannels] = useState<AlertChannel[]>(MOCK_ALERT_CHANNELS)
  const [policies, setPolicies] = useState<AlertPolicy[]>(MOCK_ALERT_POLICIES)

  // Toast
  const { toasts, toast } = useToast()

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

  // ── Event handlers ──

  const handleAcknowledge = (id: string) => {
    setEvents((prev) =>
      prev.map((e) =>
        e.id === id
          ? { ...e, status: "acknowledged", acknowledgedAt: new Date().toISOString() }
          : e
      )
    )
    toast("告警已确认")
  }

  const handleTestChannel = (id: string) => {
    const ch = channels.find((c) => c.id === id)
    toast(`测试消息已发送至 ${ch?.name ?? id}`)
  }

  const handleDeleteChannel = (id: string) => {
    const ch = channels.find((c) => c.id === id)
    setConfirm({
      open: true,
      title: "删除通道",
      description: `确认删除通道 "${ch?.name}"？此操作不可撤销。`,
      onConfirm: () => {
        setChannels((prev) => prev.filter((c) => c.id !== id))
        setConfirm((p) => ({ ...p, open: false }))
        toast(`通道 "${ch?.name}" 已删除`)
      },
    })
  }

  const handleAddChannel = () =>
    setSheet({ open: true, mode: "add-channel" })

  const handleSaveChannel = (partial: Omit<AlertChannel, "id" | "verified">) => {
    const newCh: AlertChannel = {
      ...partial,
      id: `ch-${Date.now()}`,
      verified: false,
    }
    setChannels((prev) => [...prev, newCh])
    setSheet((p) => ({ ...p, open: false }))
    toast(`通道 "${newCh.name}" 已添加`)
  }

  const handleTogglePolicy = (id: string) => {
    setPolicies((prev) =>
      prev.map((p) => (p.id === id ? { ...p, enabled: !p.enabled } : p))
    )
  }

  const handleEditPolicy = (pol: AlertPolicy) =>
    setSheet({ open: true, mode: "edit-policy", policy: pol })

  const handleDeletePolicy = (id: string) => {
    const pol = policies.find((p) => p.id === id)
    setConfirm({
      open: true,
      title: "删除策略",
      description: `确认删除策略 "${pol?.name}"？此操作不可撤销。`,
      onConfirm: () => {
        setPolicies((prev) => prev.filter((p) => p.id !== id))
        setConfirm((p) => ({ ...p, open: false }))
        toast(`策略 "${pol?.name}" 已删除`)
      },
    })
  }

  const handleAddPolicy = () =>
    setSheet({ open: true, mode: "add-policy" })

  const handleSavePolicy = (partial: Omit<AlertPolicy, "id">) => {
    if (sheet.mode === "edit-policy" && sheet.policy) {
      setPolicies((prev) =>
        prev.map((p) =>
          p.id === sheet.policy!.id ? { ...p, ...partial } : p
        )
      )
      toast("策略已更新")
    } else {
      const newPol: AlertPolicy = { ...partial, id: `pol-${Date.now()}` }
      setPolicies((prev) => [...prev, newPol])
      toast(`策略 "${newPol.name}" 已创建`)
    }
    setSheet((p) => ({ ...p, open: false }))
  }

  // ── Tab labels ──
  const TABS: { id: Tab; label: string }[] = [
    { id: "events", label: "事件历史" },
    { id: "channels", label: "告警通道" },
    { id: "policies", label: "告警策略" },
  ]

  return (
    <div className="space-y-6">
      {/* Tab navigation */}
      <div className="flex gap-1 rounded-lg border bg-muted/30 p-1" role="tablist" aria-label="告警管理">
        {TABS.map((tab) => (
          <button
            key={tab.id}
            role="tab"
            aria-selected={activeTab === tab.id}
            onClick={() => setActiveTab(tab.id)}
            className={[
              "flex-1 rounded-md px-4 py-2 text-sm font-medium transition-colors",
              activeTab === tab.id
                ? "bg-background text-foreground shadow-sm"
                : "text-muted-foreground hover:text-foreground",
            ].join(" ")}
            data-testid={`tab-${tab.id}`}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {/* Tab panels */}
      {activeTab === "events" && (
        <EventsTab events={events} onAcknowledge={handleAcknowledge} />
      )}
      {activeTab === "channels" && (
        <ChannelsTab
          channels={channels}
          onTest={handleTestChannel}
          onDelete={handleDeleteChannel}
          onAdd={handleAddChannel}
        />
      )}
      {activeTab === "policies" && (
        <PoliciesTab
          policies={policies}
          channels={channels}
          onToggle={handleTogglePolicy}
          onEdit={handleEditPolicy}
          onDelete={handleDeletePolicy}
          onAdd={handleAddPolicy}
        />
      )}

      {/* Confirm dialog */}
      <ConfirmDialog
        open={confirm.open}
        title={confirm.title}
        description={confirm.description}
        onConfirm={confirm.onConfirm}
        onCancel={() => setConfirm((p) => ({ ...p, open: false }))}
      />

      {/* Side sheet */}
      <Sheet
        open={sheet.open}
        title={
          sheet.mode === "add-channel"
            ? "添加告警通道"
            : sheet.mode === "edit-policy"
            ? "编辑告警策略"
            : "新建告警策略"
        }
        onClose={() => setSheet((p) => ({ ...p, open: false }))}
      >
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
      </Sheet>

      {/* Toast notifications */}
      <ToastContainer toasts={toasts} />
    </div>
  )
}
