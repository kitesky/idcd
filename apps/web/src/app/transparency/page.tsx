import type { Metadata } from "next"
import {
  Badge,
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui"

export const metadata: Metadata = {
  title: "透明度报告 — idcd",
  description: "idcd 平台 KMS 信任根状态、TSA 健康度、节点覆盖与平台可用率公开仪表盘",
  alternates: { canonical: "https://idcd.com/transparency" },
}

interface UptimeBlocksProps {
  uptime: number
}

function UptimeBlocks({ uptime }: UptimeBlocksProps) {
  const total = 30
  const failed = Math.round(((100 - uptime) / 100) * total)
  return (
    <div className="flex flex-wrap gap-1 mt-3">
      {Array.from({ length: total }).map((_, i) => (
        <div
          key={i}
          className={`h-4 w-4 rounded-sm ${i >= total - failed ? "bg-muted" : "bg-green-500"}`}
        />
      ))}
    </div>
  )
}

const mockData = {
  overall_status: "operational",
  last_updated: new Date().toISOString(),
  platform_uptime: { "30d": 99.97, "90d": 99.95, "365d": 99.92 },
  nodes: { total: 127, active: 124, regions: 18 },
  kms: {
    status: "operational",
    last_ceremony: "2026-01-15T10:00:00Z",
    next_ceremony: "2027-01-15T10:00:00Z",
    quorum_holders: 5,
  },
  tsa: {
    providers: [
      { name: "DigiCert", status: "operational", last_check: new Date().toISOString() },
      { name: "GlobalSign", status: "operational", last_check: new Date().toISOString() },
    ],
  },
  recent_incidents: [
    {
      date: "2026-05-10",
      title: "API 网关短暂延迟升高",
      duration_min: 12,
      severity: "low",
      resolved: true,
    },
  ],
  appeal_stats: { total: 3, resolved: 3, pending: 0, avg_resolution_h: 18.5 },
}

function statusBadge(status: string) {
  if (status === "operational") {
    return <Badge className="bg-green-500/20 text-green-400 border-green-500/30">● 运行正常</Badge>
  }
  return <Badge variant="destructive">● {status}</Badge>
}

function severityBadge(severity: string) {
  if (severity === "low") return <Badge variant="secondary">低</Badge>
  if (severity === "medium") return <Badge className="bg-yellow-500/20 text-yellow-400 border-yellow-500/30">中</Badge>
  return <Badge variant="destructive">高</Badge>
}

export default function TransparencyPage() {
  const d = mockData

  return (
    <main className="min-h-screen bg-background">
      <div className="container mx-auto px-4 py-8 max-w-5xl">
        <div className="mb-8">
          <h1 className="text-3xl font-bold tracking-tight">透明度报告</h1>
          <p className="mt-2 text-muted-foreground">
            idcd 平台实时运行状态 · 最后更新：{new Date(d.last_updated).toLocaleString("zh-CN")}
          </p>
        </div>

        <Card className="mb-6">
          <CardContent className="pt-6">
            <div
              data-testid="overall-status"
              className="flex items-center gap-3 text-xl font-semibold"
            >
              {d.overall_status === "operational" ? (
                <Badge className="text-base px-4 py-1.5 bg-green-500/20 text-green-400 border-green-500/30">
                  ● 所有系统运行正常
                </Badge>
              ) : (
                <Badge variant="destructive" className="text-base px-4 py-1.5">
                  ● 部分系统异常
                </Badge>
              )}
            </div>
          </CardContent>
        </Card>

        <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-6">
          {(
            [
              { label: "30 天可用率", value: d.platform_uptime["30d"] },
              { label: "90 天可用率", value: d.platform_uptime["90d"] },
              { label: "365 天可用率", value: d.platform_uptime["365d"] },
            ] as const
          ).map(({ label, value }) => (
            <Card key={label}>
              <CardHeader className="pb-2">
                <CardTitle className="text-sm font-medium text-muted-foreground">{label}</CardTitle>
              </CardHeader>
              <CardContent>
                <p className="text-2xl font-bold text-green-400">{value}%</p>
                <UptimeBlocks uptime={value} />
              </CardContent>
            </Card>
          ))}
        </div>

        <Card className="mb-6">
          <CardHeader>
            <CardTitle>节点覆盖</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="grid grid-cols-3 gap-4 text-center">
              <div>
                <p className="text-3xl font-bold">{d.nodes.total}</p>
                <p className="text-sm text-muted-foreground mt-1">总节点</p>
              </div>
              <div>
                <p className="text-3xl font-bold text-green-400">{d.nodes.active}</p>
                <p className="text-sm text-muted-foreground mt-1">活跃节点</p>
              </div>
              <div>
                <p className="text-3xl font-bold">{d.nodes.regions}</p>
                <p className="text-sm text-muted-foreground mt-1">覆盖地区</p>
              </div>
            </div>
          </CardContent>
        </Card>

        <Card className="mb-6" data-testid="kms-card">
          <CardHeader>
            <CardTitle>KMS 信任根</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="flex items-center justify-between">
              <span className="text-sm text-muted-foreground">状态</span>
              {statusBadge(d.kms.status)}
            </div>
            <div className="flex items-center justify-between">
              <span className="text-sm text-muted-foreground">上次密钥仪式</span>
              <span className="text-sm font-medium">
                {new Date(d.kms.last_ceremony).toLocaleDateString("zh-CN")}
              </span>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-sm text-muted-foreground">下次计划仪式</span>
              <span className="text-sm font-medium">
                {new Date(d.kms.next_ceremony).toLocaleDateString("zh-CN")}
              </span>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-sm text-muted-foreground">Quorum 持有者</span>
              <span className="text-sm font-medium">{d.kms.quorum_holders} 人（3-of-5）</span>
            </div>
          </CardContent>
        </Card>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-6" data-testid="tsa-providers">
          {d.tsa.providers.map((p) => (
            <Card key={p.name}>
              <CardHeader className="pb-2">
                <CardTitle className="text-base">{p.name}</CardTitle>
              </CardHeader>
              <CardContent className="space-y-2">
                {statusBadge(p.status)}
                <p className="text-xs text-muted-foreground mt-2">
                  最近检查：{new Date(p.last_check).toLocaleString("zh-CN")}
                </p>
              </CardContent>
            </Card>
          ))}
        </div>

        <Card className="mb-6">
          <CardHeader>
            <CardTitle>近期事故</CardTitle>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>日期</TableHead>
                  <TableHead>描述</TableHead>
                  <TableHead>持续时间</TableHead>
                  <TableHead>严重程度</TableHead>
                  <TableHead>状态</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {d.recent_incidents.map((inc, i) => (
                  <TableRow key={i}>
                    <TableCell className="text-sm">{inc.date}</TableCell>
                    <TableCell className="text-sm">{inc.title}</TableCell>
                    <TableCell className="text-sm">{inc.duration_min} 分钟</TableCell>
                    <TableCell>{severityBadge(inc.severity)}</TableCell>
                    <TableCell>
                      {inc.resolved ? (
                        <Badge variant="secondary">已解决</Badge>
                      ) : (
                        <Badge variant="destructive">处理中</Badge>
                      )}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>申诉统计</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="grid grid-cols-2 md:grid-cols-4 gap-4 text-center">
              <div>
                <p className="text-2xl font-bold">{d.appeal_stats.total}</p>
                <p className="text-sm text-muted-foreground mt-1">总申诉数</p>
              </div>
              <div>
                <p className="text-2xl font-bold text-green-400">{d.appeal_stats.resolved}</p>
                <p className="text-sm text-muted-foreground mt-1">已解决</p>
              </div>
              <div>
                <p className="text-2xl font-bold">{d.appeal_stats.pending}</p>
                <p className="text-sm text-muted-foreground mt-1">待处理</p>
              </div>
              <div>
                <p className="text-2xl font-bold">{d.appeal_stats.avg_resolution_h}h</p>
                <p className="text-sm text-muted-foreground mt-1">平均解决时间</p>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>
    </main>
  )
}
