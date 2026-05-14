"use client"

import { useState } from "react"
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

const MOCK_SCHEDULE = {
  id: "sch_demo",
  name: "工程师值班",
  rotationType: "weekly" as const,
  handoffHour: 9,
  teamId: "t_demo",
}

const MOCK_PARTICIPANTS = [
  { userId: "u_alice", name: "Alice Chen", email: "alice@idcd.com", orderIndex: 0 },
  { userId: "u_bob", name: "Bob Wang", email: "bob@idcd.com", orderIndex: 1 },
  { userId: "u_carol", name: "Carol Liu", email: "carol@idcd.com", orderIndex: 2 },
]

function getCurrentOnCallIndex(): number {
  const epoch = new Date("2024-01-01T09:00:00Z")
  const now = new Date()
  const msPerWeek = 7 * 24 * 60 * 60 * 1000
  const elapsed = now.getTime() - epoch.getTime()
  const weekIndex = Math.floor(elapsed / msPerWeek)
  return ((weekIndex % MOCK_PARTICIPANTS.length) + MOCK_PARTICIPANTS.length) % MOCK_PARTICIPANTS.length
}

function getNextHandoff(): number {
  const now = new Date()
  const epoch = new Date("2024-01-01T09:00:00Z")
  const msPerWeek = 7 * 24 * 60 * 60 * 1000
  const elapsed = now.getTime() - epoch.getTime()
  const currentWeekStart = epoch.getTime() + Math.floor(elapsed / msPerWeek) * msPerWeek
  const nextHandoff = currentWeekStart + msPerWeek
  return Math.round((nextHandoff - now.getTime()) / (60 * 60 * 1000))
}

function get7DayPreview() {
  const epoch = new Date("2024-01-01T09:00:00Z")
  const now = new Date()
  const msPerWeek = 7 * 24 * 60 * 60 * 1000
  const days = []
  for (let i = 0; i < 7; i++) {
    const day = new Date(now)
    day.setDate(day.getDate() + i)
    const elapsed = day.getTime() - epoch.getTime()
    const weekIndex = Math.floor(elapsed / msPerWeek)
    const idx = ((weekIndex % MOCK_PARTICIPANTS.length) + MOCK_PARTICIPANTS.length) % MOCK_PARTICIPANTS.length
    days.push({
      date: day,
      participant: MOCK_PARTICIPANTS[idx],
    })
  }
  return days
}

function formatDate(d: Date): string {
  return d.toLocaleDateString("zh-CN", { month: "short", day: "numeric", weekday: "short" })
}

function CreateScheduleDialog() {
  const [open, setOpen] = useState(false)
  const [name, setName] = useState("")
  const [rotationType, setRotationType] = useState("weekly")
  const [handoffHour, setHandoffHour] = useState("9")

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
                <SelectItem value="custom">自定义</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div className="grid gap-2">
            <Label htmlFor="handoff-hour">交班时间（UTC小时）</Label>
            <Input
              id="handoff-hour"
              type="number"
              min={0}
              max={23}
              value={handoffHour}
              onChange={(e) => setHandoffHour(e.target.value)}
              data-testid="handoff-hour-input"
            />
          </div>
        </div>
        <div className="flex justify-end gap-2">
          <Button variant="outline" onClick={() => setOpen(false)}>
            取消
          </Button>
          <Button
            onClick={() => setOpen(false)}
            disabled={!name.trim()}
            data-testid="create-schedule-submit"
          >
            创建
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}

function OverrideDialog() {
  const [open, setOpen] = useState(false)
  const [startDate, setStartDate] = useState("")
  const [endDate, setEndDate] = useState("")
  const [selectedUser, setSelectedUser] = useState("")

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
        <div className="grid gap-4 py-4">
          <div className="grid gap-2">
            <Label htmlFor="override-user">替换为</Label>
            <Select value={selectedUser} onValueChange={setSelectedUser}>
              <SelectTrigger id="override-user" data-testid="override-user-select">
                <SelectValue placeholder="选择值班人员" />
              </SelectTrigger>
              <SelectContent>
                {MOCK_PARTICIPANTS.map((p) => (
                  <SelectItem key={p.userId} value={p.userId}>
                    {p.name}
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
            onClick={() => setOpen(false)}
            disabled={!selectedUser || !startDate || !endDate}
            data-testid="override-submit"
          >
            确认换班
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}

export default function OncallPage() {
  const currentIdx = getCurrentOnCallIndex()
  const currentParticipant = MOCK_PARTICIPANTS[currentIdx]
  const hoursUntilHandoff = getNextHandoff()
  const preview = get7DayPreview()

  return (
    <div data-testid="oncall-page">
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight" data-testid="oncall-title">
            On-Call 排班
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            {MOCK_SCHEDULE.name} · {MOCK_SCHEDULE.rotationType === "weekly" ? "每周轮换" : "每日轮换"}
          </p>
        </div>
        <div className="flex gap-2">
          <OverrideDialog />
          <CreateScheduleDialog />
        </div>
      </div>

      <div className="grid gap-6 md:grid-cols-2">
        <Card data-testid="current-oncall-card">
          <CardHeader className="pb-3">
            <CardTitle className="flex items-center gap-2 text-base">
              <UserCheck className="h-4 w-4 text-primary" />
              当前值班
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex items-center gap-4">
              <div className="flex h-16 w-16 items-center justify-center rounded-full bg-primary/10 text-xl font-semibold text-primary">
                {currentParticipant.name.charAt(0)}
              </div>
              <div>
                <p className="text-2xl font-bold" data-testid="current-oncall-name">
                  {currentParticipant.name}
                </p>
                <p className="text-sm text-muted-foreground">{currentParticipant.email}</p>
                <div className="mt-2 flex items-center gap-1.5 text-sm text-muted-foreground">
                  <Clock className="h-3.5 w-3.5" />
                  <span data-testid="hours-until-handoff">
                    距下次交班还有 {hoursUntilHandoff} 小时
                  </span>
                </div>
              </div>
            </div>
            <div className="mt-4 flex gap-2">
              {MOCK_PARTICIPANTS.map((p, i) => (
                <Badge
                  key={p.userId}
                  variant={i === currentIdx ? "default" : "outline"}
                  className="text-xs"
                  data-testid={`participant-badge-${p.userId}`}
                >
                  {p.name.split(" ")[0]}
                </Badge>
              ))}
            </div>
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
                      {participant.name.charAt(0)}
                    </div>
                    <span className="font-medium">{participant.name}</span>
                  </div>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
