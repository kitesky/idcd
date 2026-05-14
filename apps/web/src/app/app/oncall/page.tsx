"use client"

import { useState, useEffect } from "react"
import {
  UserCheck,
  Clock,
  Plus,
  RefreshCw,
  ChevronRight,
} from "lucide-react"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { Skeleton } from "@/components/ui/skeleton"
import { apiRequest } from "@/lib/api"

// ── API types ──────────────────────────────────────────────────────────────

interface Schedule {
  id: string
  name: string
  rotation_type: "weekly" | "daily"
  start_date: string
  team_id?: string
}

interface Participant {
  id: string
  schedule_id: string
  user_id: string
  email: string
  order_index: number
}

// ── Rotation helpers ───────────────────────────────────────────────────────

function getCurrentOnCallIndex(
  startDate: string,
  rotationType: "weekly" | "daily",
  count: number,
): number {
  if (count === 0) return 0
  const epoch = new Date(startDate)
  const now = new Date()
  const ms = rotationType === "weekly" ? 7 * 24 * 60 * 60 * 1000 : 24 * 60 * 60 * 1000
  const elapsed = now.getTime() - epoch.getTime()
  const index = Math.floor(elapsed / ms)
  return ((index % count) + count) % count
}

function getNextHandoff(startDate: string, rotationType: "weekly" | "daily"): number {
  const epoch = new Date(startDate)
  const now = new Date()
  const ms = rotationType === "weekly" ? 7 * 24 * 60 * 60 * 1000 : 24 * 60 * 60 * 1000
  const elapsed = now.getTime() - epoch.getTime()
  const currentPeriodStart = epoch.getTime() + Math.floor(elapsed / ms) * ms
  const nextHandoff = currentPeriodStart + ms
  return Math.round((nextHandoff - now.getTime()) / (60 * 60 * 1000))
}

function get7DayPreview(
  startDate: string,
  rotationType: "weekly" | "daily",
  participants: Participant[],
) {
  if (participants.length === 0) return []
  const sorted = [...participants].sort((a, b) => a.order_index - b.order_index)
  const epoch = new Date(startDate)
  const now = new Date()
  const ms = rotationType === "weekly" ? 7 * 24 * 60 * 60 * 1000 : 24 * 60 * 60 * 1000
  const days = []
  for (let i = 0; i < 7; i++) {
    const day = new Date(now)
    day.setDate(day.getDate() + i)
    const elapsed = day.getTime() - epoch.getTime()
    const periodIndex = Math.floor(elapsed / ms)
    const idx = ((periodIndex % sorted.length) + sorted.length) % sorted.length
    days.push({ date: day, participant: sorted[idx] })
  }
  return days
}

function formatDate(d: Date): string {
  return d.toLocaleDateString("zh-CN", { month: "short", day: "numeric", weekday: "short" })
}

// ── Create Schedule Dialog ─────────────────────────────────────────────────

interface CreateScheduleDialogProps {
  onCreated: () => void
}

function CreateScheduleDialog({ onCreated }: CreateScheduleDialogProps) {
  const [open, setOpen] = useState(false)
  const [name, setName] = useState("")
  const [rotationType, setRotationType] = useState("weekly")
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function handleSubmit() {
    if (!name.trim()) return
    setSubmitting(true)
    setError(null)
    try {
      await apiRequest("/v1/oncall/schedules", {
        method: "POST",
        body: JSON.stringify({
          name: name.trim(),
          rotation_type: rotationType,
          start_date: new Date().toISOString(),
        }),
      })
      setOpen(false)
      setName("")
      setRotationType("weekly")
      onCreated()
    } catch (err) {
      setError(err instanceof Error ? err.message : "创建失败")
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button size="sm" data-testid="create-schedule-button">
          <Plus className="mr-2 h-4 w-4" />
          创建排班
        </Button>
      </DialogTrigger>
      <DialogContent data-testid="create-schedule-dialog">
        <DialogHeader>
          <DialogTitle>创建新排班</DialogTitle>
        </DialogHeader>
        {error && (
          <Alert variant="destructive">
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        )}
        <div className="grid gap-4 py-4">
          <div className="grid gap-2">
            <Label htmlFor="schedule-name">排班名称</Label>
            <Input
              id="schedule-name"
              placeholder="例：工程师值班"
              value={name}
              onChange={(e) => setName(e.target.value)}
              data-testid="schedule-name-input"
            />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="rotation-type">轮换方式</Label>
            <Select value={rotationType} onValueChange={setRotationType}>
              <SelectTrigger id="rotation-type" data-testid="rotation-type-select">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="daily">每日轮换</SelectItem>
                <SelectItem value="weekly">每周轮换</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>
        <div className="flex justify-end gap-2">
          <Button variant="outline" onClick={() => setOpen(false)}>
            取消
          </Button>
          <Button
            onClick={handleSubmit}
            disabled={!name.trim() || submitting}
            data-testid="create-schedule-submit"
          >
            {submitting ? "创建中…" : "创建"}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}

// ── Override Dialog ────────────────────────────────────────────────────────

interface OverrideDialogProps {
  scheduleId: string
  participants: Participant[]
}

function OverrideDialog({ scheduleId, participants }: OverrideDialogProps) {
  const [open, setOpen] = useState(false)
  const [startDate, setStartDate] = useState("")
  const [endDate, setEndDate] = useState("")
  const [selectedUser, setSelectedUser] = useState("")
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function handleSubmit() {
    if (!selectedUser || !startDate || !endDate) return
    setSubmitting(true)
    setError(null)
    try {
      await apiRequest(`/v1/oncall/schedules/${scheduleId}/overrides`, {
        method: "POST",
        body: JSON.stringify({
          user_id: selectedUser,
          start_time: new Date(startDate).toISOString(),
          end_time: new Date(endDate).toISOString(),
        }),
      })
      setOpen(false)
      setStartDate("")
      setEndDate("")
      setSelectedUser("")
    } catch (err) {
      setError(err instanceof Error ? err.message : "换班失败")
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button variant="outline" size="sm" data-testid="override-button">
          <RefreshCw className="mr-2 h-4 w-4" />
          临时换班
        </Button>
      </DialogTrigger>
      <DialogContent data-testid="override-dialog">
        <DialogHeader>
          <DialogTitle>临时换班</DialogTitle>
        </DialogHeader>
        {error && (
          <Alert variant="destructive">
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        )}
        <div className="grid gap-4 py-4">
          <div className="grid gap-2">
            <Label htmlFor="override-user">替换为</Label>
            <Select value={selectedUser} onValueChange={setSelectedUser}>
              <SelectTrigger id="override-user" data-testid="override-user-select">
                <SelectValue placeholder="选择值班人员" />
              </SelectTrigger>
              <SelectContent>
                {participants.map((p) => (
                  <SelectItem key={p.user_id} value={p.user_id}>
                    {p.email}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="grid gap-2">
            <Label htmlFor="override-start">开始时间</Label>
            <Input
              id="override-start"
              type="datetime-local"
              value={startDate}
              onChange={(e) => setStartDate(e.target.value)}
              data-testid="override-start-input"
            />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="override-end">结束时间</Label>
            <Input
              id="override-end"
              type="datetime-local"
              value={endDate}
              onChange={(e) => setEndDate(e.target.value)}
              data-testid="override-end-input"
            />
          </div>
        </div>
        <div className="flex justify-end gap-2">
          <Button variant="outline" onClick={() => setOpen(false)}>
            取消
          </Button>
          <Button
            onClick={handleSubmit}
            disabled={!selectedUser || !startDate || !endDate || submitting}
            data-testid="override-submit"
          >
            {submitting ? "提交中…" : "确认换班"}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}

// ── Main page ──────────────────────────────────────────────────────────────

export default function OncallPage() {
  const [schedule, setSchedule] = useState<Schedule | null>(null)
  const [participants, setParticipants] = useState<Participant[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  async function loadData() {
    setLoading(true)
    setError(null)
    try {
      const listRes = await apiRequest<{ data: { schedules: Schedule[] } }>(
        "/v1/oncall/schedules",
      )
      const schedules = listRes?.data?.schedules ?? []
      if (schedules.length === 0) {
        setSchedule(null)
        setParticipants([])
        return
      }
      const first = schedules[0]
      setSchedule(first)

      const partRes = await apiRequest<{ data: { participants: Participant[] } }>(
        `/v1/oncall/schedules/${first.id}/participants`,
      )
      const parts = partRes?.data?.participants ?? []
      const sorted = [...parts].sort((a, b) => a.order_index - b.order_index)
      setParticipants(sorted)
    } catch (err) {
      setError(err instanceof Error ? err.message : "加载排班数据失败")
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadData()
  }, [])

  // Derived values
  const currentIdx =
    schedule && participants.length > 0
      ? getCurrentOnCallIndex(schedule.start_date, schedule.rotation_type, participants.length)
      : 0
  const currentParticipant = participants[currentIdx] ?? null
  const hoursUntilHandoff =
    schedule ? getNextHandoff(schedule.start_date, schedule.rotation_type) : 0
  const preview =
    schedule ? get7DayPreview(schedule.start_date, schedule.rotation_type, participants) : []

  return (
    <div data-testid="oncall-page">
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight" data-testid="oncall-title">
            On-Call 排班
          </h1>
          {loading ? (
            <Skeleton className="mt-1 h-4 w-48" />
          ) : schedule ? (
            <p className="mt-1 text-sm text-muted-foreground">
              {schedule.name} · {schedule.rotation_type === "weekly" ? "每周轮换" : "每日轮换"}
            </p>
          ) : (
            <p className="mt-1 text-sm text-muted-foreground">暂无排班</p>
          )}
        </div>
        <div className="flex gap-2">
          <OverrideDialog scheduleId={schedule?.id ?? ""} participants={participants} />
          <CreateScheduleDialog onCreated={loadData} />
        </div>
      </div>

      {error && (
        <Alert variant="destructive" className="mb-6">
          <AlertDescription data-testid="oncall-error">{error}</AlertDescription>
        </Alert>
      )}

      <div className="grid gap-6 md:grid-cols-2">
        <Card data-testid="current-oncall-card">
          <CardHeader className="pb-3">
            <CardTitle className="flex items-center gap-2 text-base">
              <UserCheck className="h-4 w-4 text-primary" />
              当前值班
            </CardTitle>
          </CardHeader>
          <CardContent>
            {loading ? (
              <div className="flex items-center gap-4">
                <Skeleton className="h-16 w-16 rounded-full" />
                <div className="space-y-2">
                  <Skeleton className="h-7 w-32" />
                  <Skeleton className="h-4 w-40" />
                  <Skeleton className="h-4 w-36" />
                </div>
              </div>
            ) : currentParticipant ? (
              <>
                <div className="flex items-center gap-4">
                  <div className="flex h-16 w-16 items-center justify-center rounded-full bg-primary/10 text-xl font-semibold text-primary">
                    {currentParticipant.email.charAt(0).toUpperCase()}
                  </div>
                  <div>
                    <p className="text-2xl font-bold" data-testid="current-oncall-name">
                      {currentParticipant.email}
                    </p>
                    <p className="text-sm text-muted-foreground">{currentParticipant.user_id}</p>
                    <div className="mt-2 flex items-center gap-1.5 text-sm text-muted-foreground">
                      <Clock className="h-3.5 w-3.5" />
                      <span data-testid="hours-until-handoff">
                        距下次交班还有 {hoursUntilHandoff} 小时
                      </span>
                    </div>
                  </div>
                </div>
                <div className="mt-4 flex gap-2 flex-wrap">
                  {participants.map((p, i) => (
                    <Badge
                      key={p.user_id}
                      variant={i === currentIdx ? "default" : "outline"}
                      className="text-xs"
                      data-testid={`participant-badge-${p.user_id}`}
                    >
                      {p.email.split("@")[0]}
                    </Badge>
                  ))}
                </div>
              </>
            ) : (
              <p className="text-sm text-muted-foreground">暂无值班人员</p>
            )}
          </CardContent>
        </Card>

        <Card data-testid="schedule-preview-card">
          <CardHeader className="pb-3">
            <CardTitle className="flex items-center gap-2 text-base">
              <ChevronRight className="h-4 w-4 text-primary" />
              未来 7 天排班
            </CardTitle>
          </CardHeader>
          <CardContent>
            {loading ? (
              <div className="space-y-2">
                {Array.from({ length: 7 }).map((_, i) => (
                  <Skeleton key={i} className="h-8 w-full" />
                ))}
              </div>
            ) : preview.length > 0 ? (
              <div className="space-y-2" data-testid="preview-list">
                {preview.map(({ date, participant }, i) => (
                  <div
                    key={i}
                    className="flex items-center justify-between rounded-md px-2 py-1.5 text-sm hover:bg-muted/50"
                    data-testid={`preview-day-${i}`}
                  >
                    <span className="text-muted-foreground w-28 shrink-0">
                      {formatDate(date)}
                    </span>
                    <div className="flex items-center gap-2">
                      <div className="flex h-6 w-6 items-center justify-center rounded-full bg-primary/10 text-xs text-primary">
                        {participant.email.charAt(0).toUpperCase()}
                      </div>
                      <span className="font-medium">{participant.email}</span>
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <p className="text-sm text-muted-foreground">暂无排班数据</p>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
