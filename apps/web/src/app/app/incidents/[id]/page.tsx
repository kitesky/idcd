"use client"

import { useEffect, useState, useCallback } from "react"
import { useParams } from "next/navigation"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Separator } from "@/components/ui/separator"
import { Skeleton } from "@/components/ui/skeleton"
import { apiRequest } from "@/lib/api"

// ─── Types ────────────────────────────────────────────────────────────────────

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
  alert_event_id: string
  monitor_id: string
  title: string
  status: string
  severity: string
  impact: string
  timeline: TimelineEntry[]
  root_cause: string
  resolution: string
  action_items: ActionItem[]
  created_at: string
  updated_at: string
}

// ─── Severity variant map ─────────────────────────────────────────────────────

const severityVariant: Record<string, "destructive" | "secondary" | "outline" | "default"> = {
  critical: "destructive",
  high: "destructive",
  medium: "secondary",
  low: "outline",
}

// ─── Skeleton loader ──────────────────────────────────────────────────────────

function PostmortemSkeleton() {
  return (
    <div className="min-h-screen bg-background" data-testid="postmortem-detail-page">
      <div className="container mx-auto max-w-3xl px-4 py-8">
        <div className="mb-6 space-y-2">
          <Skeleton className="h-5 w-16" />
          <Skeleton className="h-8 w-3/4" />
          <Skeleton className="h-4 w-1/2" />
        </div>
        <div className="space-y-4">
          {Array.from({ length: 5 }).map((_, i) => (
            <Card key={i}>
              <CardHeader>
                <Skeleton className="h-4 w-24" />
              </CardHeader>
              <CardContent>
                <Skeleton className="h-16 w-full" />
              </CardContent>
            </Card>
          ))}
        </div>
      </div>
    </div>
  )
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export default function PostmortemDetailPage() {
  const params = useParams<{ id: string }>()
  const id = params?.id

  const [pm, setPm] = useState<PostmortemDetail | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const fetchPostmortem = useCallback(async () => {
    if (!id) return
    setLoading(true)
    setError(null)
    try {
      const res = await apiRequest<{ data: PostmortemDetail }>(
        `/v1/incidents/${id}/postmortem`
      )
      setPm(res.data)
    } catch (err: unknown) {
      if (err instanceof Error) {
        // Treat "not found" / 404-like messages as a distinct state
        if (
          err.message.toLowerCase().includes("not found") ||
          err.message.includes("404")
        ) {
          setError("NOT_FOUND")
        } else {
          setError(err.message || "加载失败，请稍后重试")
        }
      } else {
        setError("加载失败，请稍后重试")
      }
    } finally {
      setLoading(false)
    }
  }, [id])

  useEffect(() => {
    fetchPostmortem()
  }, [fetchPostmortem])

  // ── Loading ──
  if (loading) {
    return <PostmortemSkeleton />
  }

  // ── 404 ──
  if (error === "NOT_FOUND") {
    return (
      <div className="min-h-screen bg-background" data-testid="postmortem-detail-page">
        <div className="container mx-auto max-w-3xl px-4 py-8">
          <Alert variant="destructive" data-testid="postmortem-not-found">
            <AlertTitle>未找到复盘记录</AlertTitle>
            <AlertDescription>
              该故障的复盘记录不存在或尚未生成，请返回故障列表。
            </AlertDescription>
          </Alert>
        </div>
      </div>
    )
  }

  // ── Generic error ──
  if (error) {
    return (
      <div className="min-h-screen bg-background" data-testid="postmortem-detail-page">
        <div className="container mx-auto max-w-3xl px-4 py-8">
          <Alert variant="destructive" data-testid="postmortem-error">
            <AlertTitle>加载失败</AlertTitle>
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        </div>
      </div>
    )
  }

  if (!pm) return null

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
            复盘状态：{pm.status} · 生成时间：{pm.created_at}
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
              <p className="text-sm">{pm.root_cause}</p>
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
                {pm.action_items.map((a, i) => (
                  <li key={i}>
                    <Separator className="mb-2" />
                    <p className="font-medium">{a.item}</p>
                    <p className="text-muted-foreground">
                      负责人：{a.owner} · 截止：{a.due_date}
                    </p>
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
