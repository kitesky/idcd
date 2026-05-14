"use client"

import { useState, useEffect } from "react"
import { useRouter } from "next/navigation"
import { FileWarning, Clock, Zap, AlertCircle } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { apiRequest } from "@/lib/api"

interface Incident {
  alert_event_id: string
  monitor_id: string
  monitor_name: string
  status: string
  severity: string
  started_at: string
  resolved_at: string | null
  has_postmortem: boolean
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

function formatDate(isoString: string | null): string {
  if (!isoString) return "—"
  const d = new Date(isoString)
  if (isNaN(d.getTime())) return isoString
  const pad = (n: number) => String(n).padStart(2, "0")
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

function IncidentsTableSkeleton() {
  return (
    <div className="space-y-2" data-testid="incidents-skeleton">
      {[1, 2, 3, 4, 5].map((i) => (
        <div key={i} className="flex items-center gap-4 py-3 px-1">
          <Skeleton className="h-4 w-32" />
          <Skeleton className="h-4 w-36" />
          <Skeleton className="h-4 w-20" />
          <Skeleton className="h-5 w-12" />
          <Skeleton className="h-5 w-14" />
          <Skeleton className="h-8 w-20" />
        </div>
      ))}
    </div>
  )
}

export default function IncidentsPage() {
  const router = useRouter()
  const [incidents, setIncidents] = useState<Incident[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [generating, setGenerating] = useState<string | null>(null)
  const [generateError, setGenerateError] = useState<string | null>(null)

  useEffect(() => {
    async function fetchIncidents() {
      setLoading(true)
      setError(null)
      try {
        const res = await apiRequest<{ data: { incidents: Incident[] } }>("/v1/reports/incidents")
        setIncidents(res.data.incidents ?? [])
      } catch (err) {
        setError(err instanceof Error ? err.message : "加载失败，请刷新重试")
      } finally {
        setLoading(false)
      }
    }
    fetchIncidents()
  }, [])

  async function handleGenerate(alertEventId: string) {
    setGenerating(alertEventId)
    setGenerateError(null)
    try {
      const res = await apiRequest<{ data: { id: string; title: string } }>("/v1/postmortems/draft", {
        method: "POST",
        body: JSON.stringify({ alert_event_id: alertEventId }),
      })
      router.push(`/app/incidents/${res.data.id}`)
    } catch (err) {
      setGenerateError(err instanceof Error ? err.message : "生成复盘失败，请重试")
      setGenerating(null)
    }
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

        {error && (
          <Alert variant="destructive" className="mb-6" data-testid="incidents-error-alert">
            <AlertCircle className="h-4 w-4" />
            <AlertTitle>加载失败</AlertTitle>
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        )}

        {generateError && (
          <Alert variant="destructive" className="mb-6" data-testid="generate-error-alert">
            <AlertCircle className="h-4 w-4" />
            <AlertTitle>操作失败</AlertTitle>
            <AlertDescription>{generateError}</AlertDescription>
          </Alert>
        )}

        <Card data-testid="incidents-table-card">
          <CardHeader>
            <CardTitle>告警事件</CardTitle>
          </CardHeader>
          <CardContent>
            {loading ? (
              <IncidentsTableSkeleton />
            ) : incidents.length === 0 ? (
              <div className="flex flex-col items-center justify-center py-12 text-center" data-testid="incidents-empty-state">
                <p className="text-sm text-muted-foreground">暂无故障记录</p>
              </div>
            ) : (
              <Table data-testid="incidents-table">
                <TableHeader>
                  <TableRow>
                    <TableHead>监控名</TableHead>
                    <TableHead>开始时间</TableHead>
                    <TableHead>严重程度</TableHead>
                    <TableHead>复盘状态</TableHead>
                    <TableHead>操作</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {incidents.map((incident) => (
                    <TableRow key={incident.alert_event_id} data-testid={`incident-row-${incident.alert_event_id}`}>
                      <TableCell className="font-medium">{incident.monitor_name}</TableCell>
                      <TableCell>
                        <span className="flex items-center gap-1 text-sm text-muted-foreground">
                          <Clock className="h-3 w-3" />
                          {formatDate(incident.started_at)}
                        </span>
                      </TableCell>
                      <TableCell>
                        <Badge
                          variant={severityVariant[incident.severity] ?? "outline"}
                          data-testid={`severity-badge-${incident.alert_event_id}`}
                        >
                          {severityLabel[incident.severity] ?? incident.severity}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        {incident.has_postmortem ? (
                          <Badge variant="secondary" data-testid={`postmortem-status-${incident.alert_event_id}`}>
                            已生成
                          </Badge>
                        ) : (
                          <Badge variant="outline" data-testid={`postmortem-status-${incident.alert_event_id}`}>
                            未生成
                          </Badge>
                        )}
                      </TableCell>
                      <TableCell>
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => handleGenerate(incident.alert_event_id)}
                          disabled={generating === incident.alert_event_id}
                          data-testid={`generate-btn-${incident.alert_event_id}`}
                        >
                          <Zap className="mr-1 h-3 w-3" />
                          {generating === incident.alert_event_id ? "生成中..." : "生成复盘"}
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
