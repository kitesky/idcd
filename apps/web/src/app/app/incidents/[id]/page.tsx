"use client"

import { useEffect, useState, useCallback } from "react"
import { useParams } from "next/navigation"
import { toast } from "sonner"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Separator } from "@/components/ui/separator"
import { Skeleton } from "@/components/ui/skeleton"
import { Textarea } from "@/components/ui/textarea"
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

interface PostmortemDraft {
  title: string
  impact: string
  root_cause: string
  resolution: string
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
    <div className="max-w-3xl space-y-4" data-testid="postmortem-detail-page">
      <div className="mb-6 space-y-2">
        <Skeleton className="h-5 w-16" />
        <Skeleton className="h-8 w-3/4" />
        <Skeleton className="h-4 w-1/2" />
      </div>
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
  )
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export default function PostmortemDetailPage() {
  const params = useParams<{ id: string }>()
  const id = params?.id

  const [pm, setPm] = useState<PostmortemDetail | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState<PostmortemDraft>({
    title: "",
    impact: "",
    root_cause: "",
    resolution: "",
  })
  const [saving, setSaving] = useState(false)

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

  function startEdit() {
    if (!pm) return
    setDraft({
      title: pm.title,
      impact: pm.impact,
      root_cause: pm.root_cause,
      resolution: pm.resolution,
    })
    setEditing(true)
  }

  function cancelEdit() {
    setEditing(false)
  }

  async function saveEdit() {
    if (!id) return
    setSaving(true)
    try {
      await apiRequest(`/v1/incidents/${id}/postmortem`, {
        method: "PATCH",
        body: JSON.stringify(draft),
      })
      await fetchPostmortem()
      setEditing(false)
      toast.success("复盘已保存")
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : "保存失败，请稍后重试"
      toast.error(msg)
    } finally {
      setSaving(false)
    }
  }

  // ── Loading ──
  if (loading) {
    return <PostmortemSkeleton />
  }

  // ── 404 ──
  if (error === "NOT_FOUND") {
    return (
      <div className="max-w-3xl" data-testid="postmortem-detail-page">
        <Alert variant="destructive" data-testid="postmortem-not-found">
          <AlertTitle>未找到复盘记录</AlertTitle>
          <AlertDescription>
            该故障的复盘记录不存在或尚未生成，请返回故障列表。
          </AlertDescription>
        </Alert>
      </div>
    )
  }

  // ── Generic error ──
  if (error) {
    return (
      <div className="max-w-3xl" data-testid="postmortem-detail-page">
        <Alert variant="destructive" data-testid="postmortem-error">
          <AlertTitle>加载失败</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      </div>
    )
  }

  if (!pm) return null

  return (
    <div className="max-w-3xl space-y-4" data-testid="postmortem-detail-page">
      {/* ── Header ── */}
      <div className="mb-2">
        <div className="flex items-start justify-between gap-4">
          <div className="flex-1 space-y-1">
            <Badge variant={severityVariant[pm.severity] ?? "outline"} className="mb-2">
              {pm.severity}
            </Badge>
            {editing ? (
              <div className="space-y-1">
                <Label htmlFor="edit-title" className="sr-only">标题</Label>
                <Input
                  id="edit-title"
                  value={draft.title}
                  onChange={(e) => setDraft((d) => ({ ...d, title: e.target.value }))}
                  className="text-xl font-bold"
                  data-testid="edit-title-input"
                />
              </div>
            ) : (
              <h1 className="text-2xl font-bold tracking-tight" data-testid="postmortem-title">
                {pm.title}
              </h1>
            )}
            <p className="mt-1 text-sm text-muted-foreground">
              复盘状态：{pm.status} · 生成时间：{pm.created_at}
            </p>
          </div>

          {/* ── Edit / Save / Cancel buttons ── */}
          <div className="flex shrink-0 gap-2">
            {editing ? (
              <>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={cancelEdit}
                  disabled={saving}
                  data-testid="cancel-edit-btn"
                >
                  取消
                </Button>
                <Button
                  size="sm"
                  onClick={saveEdit}
                  disabled={saving}
                  data-testid="save-edit-btn"
                >
                  {saving ? "保存中…" : "保存"}
                </Button>
              </>
            ) : (
              <Button
                variant="outline"
                size="sm"
                onClick={startEdit}
                data-testid="start-edit-btn"
              >
                编辑
              </Button>
            )}
          </div>
        </div>
      </div>

      <div className="space-y-4">
        {/* ── Impact ── */}
        <Card data-testid="impact-card">
          <CardHeader>
            <CardTitle className="text-base">影响范围</CardTitle>
          </CardHeader>
          <CardContent>
            {editing ? (
              <Textarea
                value={draft.impact}
                onChange={(e) => setDraft((d) => ({ ...d, impact: e.target.value }))}
                rows={3}
                data-testid="edit-impact-textarea"
              />
            ) : (
              <p>{pm.impact}</p>
            )}
          </CardContent>
        </Card>

        {/* ── Timeline (read-only) ── */}
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

        {/* ── Root Cause ── */}
        <Card data-testid="rootcause-card">
          <CardHeader>
            <CardTitle className="text-base">根因分析</CardTitle>
          </CardHeader>
          <CardContent>
            {editing ? (
              <Textarea
                value={draft.root_cause}
                onChange={(e) => setDraft((d) => ({ ...d, root_cause: e.target.value }))}
                rows={4}
                data-testid="edit-root-cause-textarea"
              />
            ) : (
              <p className="text-sm">{pm.root_cause}</p>
            )}
          </CardContent>
        </Card>

        {/* ── Resolution ── */}
        <Card data-testid="resolution-card">
          <CardHeader>
            <CardTitle className="text-base">处置方案</CardTitle>
          </CardHeader>
          <CardContent>
            {editing ? (
              <Textarea
                value={draft.resolution}
                onChange={(e) => setDraft((d) => ({ ...d, resolution: e.target.value }))}
                rows={4}
                data-testid="edit-resolution-textarea"
              />
            ) : (
              <p className="text-sm">{pm.resolution || "—"}</p>
            )}
          </CardContent>
        </Card>

        {/* ── Action Items (read-only) ── */}
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
  )
}
