"use client"

import { useState } from "react"
import { FileWarning, Clock, Zap } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"

interface TimelineEntry {
  time: string
  event: string
}

interface ActionItem {
  item: string
  owner: string
  due_date: string
}

interface PostmortemDraft {
  title: string
  severity: string
  impact: string
  timeline: TimelineEntry[]
  root_cause: string
  resolution: string
  action_items: ActionItem[]
}

interface IncidentRow {
  id: string
  monitorName: string
  startedAt: string
  duration: string
  severity: "low" | "medium" | "high" | "critical"
  status: "firing" | "resolved"
  hasPostmortem: boolean
}

const MOCK_INCIDENTS: IncidentRow[] = [
  {
    id: "ev_001",
    monitorName: "API Gateway",
    startedAt: "2026-05-13 14:23",
    duration: "47 分钟",
    severity: "high",
    status: "resolved",
    hasPostmortem: true,
  },
  {
    id: "ev_002",
    monitorName: "Payment Service",
    startedAt: "2026-05-12 09:11",
    duration: "3 小时 12 分钟",
    severity: "critical",
    status: "resolved",
    hasPostmortem: false,
  },
  {
    id: "ev_003",
    monitorName: "Auth Service",
    startedAt: "2026-05-11 22:05",
    duration: "8 分钟",
    severity: "low",
    status: "resolved",
    hasPostmortem: false,
  },
  {
    id: "ev_004",
    monitorName: "Database Primary",
    startedAt: "2026-05-10 03:47",
    duration: "22 分钟",
    severity: "medium",
    status: "resolved",
    hasPostmortem: true,
  },
  {
    id: "ev_005",
    monitorName: "CDN Edge",
    startedAt: "2026-05-09 11:30",
    duration: "2 小时 5 分钟",
    severity: "critical",
    status: "resolved",
    hasPostmortem: false,
  },
]

const MOCK_DRAFT: PostmortemDraft = {
  title: "[high] API Gateway 服务中断（47 分钟）",
  severity: "high",
  impact: "API Gateway（http）检测到异常，影响持续约 47 分钟",
  timeline: [
    { time: "2026-05-13T14:23:00Z", event: "故障开始" },
    { time: "2026-05-13T15:10:00Z", event: "故障结束" },
  ],
  root_cause: "[待补充] 初步判断为基础设施异常，具体根因需进一步分析",
  resolution: "故障于 2026-05-13T15:10:00Z 恢复，共持续 47 分钟",
  action_items: [
    { item: "检查服务器负载", owner: "待指定", due_date: "2026-05-21" },
    { item: "增加健康检查超时重试", owner: "待指定", due_date: "2026-05-21" },
    { item: "验证回滚计划", owner: "待指定", due_date: "2026-05-21" },
  ],
}

const severityVariant: Record<string, "destructive" | "secondary" | "outline" | "default"> = {
  critical: "destructive",
  high: "destructive",
  medium: "secondary",
  low: "outline",
}

const severityLabel: Record<string, string> = {
  critical: "严重",
  high: "高",
  medium: "中",
  low: "低",
}

export default function IncidentsPage() {
  const [draft, setDraft] = useState<PostmortemDraft | null>(null)
  const [dialogOpen, setDialogOpen] = useState(false)
  const [generating, setGenerating] = useState<string | null>(null)

  function handleGenerate(incident: IncidentRow) {
    setGenerating(incident.id)
    setTimeout(() => {
      setDraft({
        ...MOCK_DRAFT,
        title: `[${incident.severity}] ${incident.monitorName} 服务中断（${incident.duration}）`,
        impact: `${incident.monitorName}（http）检测到异常，影响持续约 ${incident.duration}`,
      })
      setGenerating(null)
      setDialogOpen(true)
    }, 600)
  }

  return (
    <div className="min-h-screen bg-background" data-testid="incidents-page">
      <div className="container mx-auto px-4 py-8">
        <div className="mb-8 flex items-center gap-3">
          <FileWarning className="h-7 w-7 text-muted-foreground" />
          <div>
            <h1 className="text-3xl font-bold tracking-tight">故障记录</h1>
            <p className="mt-1 text-muted-foreground">
              查看历史告警事件，自动生成事故复盘草稿
            </p>
          </div>
        </div>

        <Card data-testid="incidents-table-card">
          <CardHeader>
            <CardTitle>告警事件</CardTitle>
          </CardHeader>
          <CardContent>
            <Table data-testid="incidents-table">
              <TableHeader>
                <TableRow>
                  <TableHead>监控名</TableHead>
                  <TableHead>开始时间</TableHead>
                  <TableHead>持续时间</TableHead>
                  <TableHead>严重程度</TableHead>
                  <TableHead>复盘状态</TableHead>
                  <TableHead>操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {MOCK_INCIDENTS.map((incident) => (
                  <TableRow key={incident.id} data-testid={`incident-row-${incident.id}`}>
                    <TableCell className="font-medium">{incident.monitorName}</TableCell>
                    <TableCell>
                      <span className="flex items-center gap-1 text-sm text-muted-foreground">
                        <Clock className="h-3 w-3" />
                        {incident.startedAt}
                      </span>
                    </TableCell>
                    <TableCell>{incident.duration}</TableCell>
                    <TableCell>
                      <Badge
                        variant={severityVariant[incident.severity]}
                        data-testid={`severity-badge-${incident.id}`}
                      >
                        {severityLabel[incident.severity]}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      {incident.hasPostmortem ? (
                        <Badge variant="secondary" data-testid={`postmortem-status-${incident.id}`}>
                          已生成
                        </Badge>
                      ) : (
                        <Badge variant="outline" data-testid={`postmortem-status-${incident.id}`}>
                          未生成
                        </Badge>
                      )}
                    </TableCell>
                    <TableCell>
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => handleGenerate(incident)}
                        disabled={generating === incident.id}
                        data-testid={`generate-btn-${incident.id}`}
                      >
                        <Zap className="mr-1 h-3 w-3" />
                        {generating === incident.id ? "生成中..." : "生成复盘"}
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      </div>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="max-w-2xl" data-testid="postmortem-dialog">
          <DialogHeader>
            <DialogTitle data-testid="postmortem-dialog-title">
              {draft?.title ?? "复盘草稿"}
            </DialogTitle>
          </DialogHeader>
          {draft && (
            <div className="space-y-4 text-sm">
              <div>
                <p className="font-semibold text-muted-foreground">严重程度</p>
                <Badge variant={severityVariant[draft.severity]} className="mt-1">
                  {severityLabel[draft.severity] ?? draft.severity}
                </Badge>
              </div>
              <div>
                <p className="font-semibold text-muted-foreground">影响范围</p>
                <p className="mt-1">{draft.impact}</p>
              </div>
              <div>
                <p className="font-semibold text-muted-foreground">时间线</p>
                <ul className="mt-1 space-y-1">
                  {draft.timeline.map((t, i) => (
                    <li key={i} className="flex gap-2">
                      <span className="text-muted-foreground">{t.time}</span>
                      <span>{t.event}</span>
                    </li>
                  ))}
                </ul>
              </div>
              <div>
                <p className="font-semibold text-muted-foreground">根因分析</p>
                <p className="mt-1">{draft.root_cause}</p>
              </div>
              <div>
                <p className="font-semibold text-muted-foreground">处置方案</p>
                <p className="mt-1">{draft.resolution || "—"}</p>
              </div>
              <div>
                <p className="font-semibold text-muted-foreground">改进措施</p>
                <ul className="mt-1 space-y-1">
                  {draft.action_items.map((a, i) => (
                    <li key={i} className="flex gap-2">
                      <span>•</span>
                      <span>{a.item}（负责人：{a.owner}，截止：{a.due_date}）</span>
                    </li>
                  ))}
                </ul>
              </div>
            </div>
          )}
        </DialogContent>
      </Dialog>
    </div>
  )
}
