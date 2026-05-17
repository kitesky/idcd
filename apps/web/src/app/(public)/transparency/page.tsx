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
import { Alert, AlertDescription } from "@/components/ui/alert"

export const metadata: Metadata = {
  title: "透明度报告 — idcd",
  description: "idcd 平台 KMS 信任根状态、TSA 健康度、节点覆盖与平台可用率公开仪表盘",
  alternates: { canonical: "https://idcd.com/transparency" },
}

const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

interface TransparencyData {
  overall_status: string
  last_updated?: string
  platform_uptime: { "30d": number; "90d": number; "365d": number }
  nodes: { total: number; online?: number; active?: number; tier1?: number; regions?: number }
  kms: {
    status: string
    provider?: string
    last_ceremony?: string
    next_ceremony?: string
    quorum_holders?: number
  }
  tsa: {
    providers: Array<{ name: string; status: string; last_check?: string }>
  }
  recent_incidents: Array<{
    date: string
    title: string
    duration_min: number
    severity: string
    resolved: boolean
  }>
  appeal_stats: {
    total: number
    resolved: number
    pending?: number
    avg_resolution_h?: number
    avg_hours?: number
  }
}

async function getTransparencyData(): Promise<{ data: TransparencyData } | null> {
  try {
    const res = await fetch(`${API_BASE}/v1/transparency`, {
      next: { revalidate: 60 },
    })
    if (!res.ok) return null
    return res.json()
  } catch {
    return null
  }
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

export default async function TransparencyPage() {
  const result = await getTransparencyData()

  if (!result) {
    return (
      <main className="min-h-screen bg-background">
        <div className="container mx-auto px-4 py-8 max-w-5xl">
          <div className="mb-8">
            <h1 className="text-3xl font-bold tracking-tight">透明度报告</h1>
          </div>
          <Alert variant="destructive" data-testid="error-state">
            <AlertDescription>
              暂时无法加载透明度报告数据，请稍后刷新页面重试。
            </AlertDescription>
          </Alert>
        </div>
      </main>
    )
  }

  const d = result.data
  const lastUpdated = d.last_updated ?? new Date().toISOString()
  const activeNodes = d.nodes.online ?? d.nodes.active ?? 0
  const regionsOrTier1 = d.nodes.regions ?? d.nodes.tier1 ?? 0
  const avgResolutionH = d.appeal_stats.avg_resolution_h ?? d.appeal_stats.avg_hours ?? 0
  const pendingAppeals = d.appeal_stats.pending ?? (d.appeal_stats.total - d.appeal_stats.resolved)

  return (
    <main className="min-h-screen bg-background">
      <div className="container mx-auto px-4 py-8 max-w-5xl">
        <div className="mb-8">
          <h1 className="text-3xl font-bold tracking-tight">透明度报告</h1>
          <p className="mt-2 text-muted-foreground">
            idcd 平台实时运行状态 · 最后更新：{new Date(lastUpdated).toLocaleString("zh-CN")}
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
                <p className="text-3xl font-bold text-green-400">{activeNodes}</p>
                <p className="text-sm text-muted-foreground mt-1">活跃节点</p>
              </div>
              <div>
                <p className="text-3xl font-bold">{regionsOrTier1}</p>
                <p className="text-sm text-muted-foreground mt-1">{d.nodes.regions != null ? "覆盖地区" : "T1 节点"}</p>
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
            {d.kms.provider && (
              <div className="flex items-center justify-between">
                <span className="text-sm text-muted-foreground">服务商</span>
                <span className="text-sm font-medium">{d.kms.provider}</span>
              </div>
            )}
            {d.kms.last_ceremony && (
              <div className="flex items-center justify-between">
                <span className="text-sm text-muted-foreground">上次密钥仪式</span>
                <span className="text-sm font-medium">
                  {new Date(d.kms.last_ceremony).toLocaleDateString("zh-CN")}
                </span>
              </div>
            )}
            {d.kms.next_ceremony && (
              <div className="flex items-center justify-between">
                <span className="text-sm text-muted-foreground">下次计划仪式</span>
                <span className="text-sm font-medium">
                  {new Date(d.kms.next_ceremony).toLocaleDateString("zh-CN")}
                </span>
              </div>
            )}
            {d.kms.quorum_holders != null && (
              <div className="flex items-center justify-between">
                <span className="text-sm text-muted-foreground">Quorum 持有者</span>
                <span className="text-sm font-medium">{d.kms.quorum_holders} 人（3-of-5）</span>
              </div>
            )}
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
                {p.last_check && (
                  <p className="text-xs text-muted-foreground mt-2">
                    最近检查：{new Date(p.last_check).toLocaleString("zh-CN")}
                  </p>
                )}
              </CardContent>
            </Card>
          ))}
        </div>

        <Card className="mb-6">
          <CardHeader>
            <CardTitle>近期事故</CardTitle>
          </CardHeader>
          <CardContent>
            {d.recent_incidents.length === 0 ? (
              <p className="text-sm text-muted-foreground text-center py-4">近期无事故记录</p>
            ) : (
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
            )}
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
                <p className="text-2xl font-bold">{pendingAppeals}</p>
                <p className="text-sm text-muted-foreground mt-1">待处理</p>
              </div>
              <div>
                <p className="text-2xl font-bold">{avgResolutionH}h</p>
                <p className="text-sm text-muted-foreground mt-1">平均解决时间</p>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>
    </main>
  )
}
