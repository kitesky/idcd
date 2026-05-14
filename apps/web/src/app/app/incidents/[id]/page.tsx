import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Separator } from "@/components/ui/separator"

interface TimelineEntry {
  time: string
  event: string
}

interface ActionItem {
  item: string
  owner: string
  due_date: string
}

interface PostmortemDetail {
  id: string
  alertEventId: string
  monitorId: string
  title: string
  status: string
  severity: string
  impact: string
  timeline: TimelineEntry[]
  rootCause: string
  resolution: string
  actionItems: ActionItem[]
  createdAt: string
}

const MOCK_DETAIL: PostmortemDetail = {
  id: "pm_mock001",
  alertEventId: "ev_001",
  monitorId: "mon_api",
  title: "[high] API Gateway 服务中断（47 分钟）",
  status: "draft",
  severity: "high",
  impact: "API Gateway（http）检测到异常，影响持续约 47 分钟",
  timeline: [
    { time: "2026-05-13T14:23:00Z", event: "故障开始" },
    { time: "2026-05-13T15:10:00Z", event: "故障结束" },
  ],
  rootCause: "[待补充] 初步判断为基础设施异常，具体根因需进一步分析",
  resolution: "故障于 2026-05-13T15:10:00Z 恢复，共持续 47 分钟",
  actionItems: [
    { item: "检查服务器负载", owner: "待指定", due_date: "2026-05-21" },
    { item: "增加健康检查超时重试", owner: "待指定", due_date: "2026-05-21" },
    { item: "验证回滚计划", owner: "待指定", due_date: "2026-05-21" },
  ],
  createdAt: "2026-05-13T15:12:00Z",
}

const severityVariant: Record<string, "destructive" | "secondary" | "outline" | "default"> = {
  critical: "destructive",
  high: "destructive",
  medium: "secondary",
  low: "outline",
}

export default function PostmortemDetailPage({ params: _params }: { params: { id: string } }) {
  const pm = MOCK_DETAIL

  return (
    <div className="min-h-screen bg-background" data-testid="postmortem-detail-page">
      <div className="container mx-auto max-w-3xl px-4 py-8">
        <div className="mb-6">
          <Badge variant={severityVariant[pm.severity] ?? "outline"} className="mb-2">
            {pm.severity}
          </Badge>
          <h1 className="text-2xl font-bold tracking-tight" data-testid="postmortem-title">
            {pm.title}
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            复盘状态：{pm.status} · 生成时间：{pm.createdAt}
          </p>
        </div>

        <div className="space-y-4">
          <Card data-testid="impact-card">
            <CardHeader>
              <CardTitle className="text-base">影响范围</CardTitle>
            </CardHeader>
            <CardContent>
              <p>{pm.impact}</p>
            </CardContent>
          </Card>

          <Card data-testid="timeline-card">
            <CardHeader>
              <CardTitle className="text-base">时间线</CardTitle>
            </CardHeader>
            <CardContent>
              <ul className="space-y-2 text-sm">
                {pm.timeline.map((t, i) => (
                  <li key={i} className="flex gap-3">
                    <span className="text-muted-foreground">{t.time}</span>
                    <span>{t.event}</span>
                  </li>
                ))}
              </ul>
            </CardContent>
          </Card>

          <Card data-testid="rootcause-card">
            <CardHeader>
              <CardTitle className="text-base">根因分析</CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-sm">{pm.rootCause}</p>
            </CardContent>
          </Card>

          <Card data-testid="resolution-card">
            <CardHeader>
              <CardTitle className="text-base">处置方案</CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-sm">{pm.resolution || "—"}</p>
            </CardContent>
          </Card>

          <Card data-testid="action-items-card">
            <CardHeader>
              <CardTitle className="text-base">改进措施</CardTitle>
            </CardHeader>
            <CardContent>
              <ul className="space-y-2 text-sm">
                {pm.actionItems.map((a, i) => (
                  <li key={i}>
                    <Separator className="mb-2" />
                    <p className="font-medium">{a.item}</p>
                    <p className="text-muted-foreground">负责人：{a.owner} · 截止：{a.due_date}</p>
                  </li>
                ))}
              </ul>
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  )
}
