"use client"

import { useState, useEffect, useCallback } from "react"
import Link from "next/link"
import { useParams } from "next/navigation"
import { ArrowLeft, Plus, Trash2, AlertCircle, ExternalLink } from "lucide-react"
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  CardDescription,
} from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Badge } from "@/components/ui/badge"
import { Switch } from "@/components/ui/switch"
import { Skeleton } from "@/components/ui/skeleton"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import { apiRequest } from "@/lib/api"

// ── Types ────────────────────────────────────────────────────────────────────

interface StatusPage {
  id: string
  name: string
  slug: string
  is_public: boolean
  overall_status: string
  created_at: string
}

interface ApiMonitor {
  id: string
  name: string
  type: string
  target: string
  status: string
  uptime_percent: number
  last_checked_at: string
  interval_seconds: number
}

interface StatusPageMonitor {
  id: string
  monitor_id: string
  name: string
  type: string
  target: string
  status: string
  position: number
}

// ── Loading Skeleton ─────────────────────────────────────────────────────────

function PageSkeleton() {
  return (
    <div className="space-y-6">
      <div className="space-y-2">
        <Skeleton className="h-5 w-48" />
        <Skeleton className="h-8 w-64" />
      </div>
      <Card>
        <CardHeader>
          <Skeleton className="h-5 w-32" />
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Skeleton className="h-4 w-16" />
            <Skeleton className="h-9 w-full" />
          </div>
          <div className="space-y-2">
            <Skeleton className="h-4 w-12" />
            <Skeleton className="h-9 w-full" />
          </div>
          <div className="flex items-center justify-between">
            <Skeleton className="h-4 w-24" />
            <Skeleton className="h-6 w-10" />
          </div>
          <Skeleton className="h-9 w-20" />
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <Skeleton className="h-5 w-32" />
        </CardHeader>
        <CardContent className="space-y-3">
          {[1, 2].map((i) => (
            <div key={i} className="flex items-center justify-between rounded-md border p-3">
              <div className="space-y-1">
                <Skeleton className="h-4 w-40" />
                <Skeleton className="h-3 w-32" />
              </div>
              <Skeleton className="h-8 w-8" />
            </div>
          ))}
        </CardContent>
      </Card>
    </div>
  )
}

// ── Main Page ────────────────────────────────────────────────────────────────

export default function StatusPageDetailPage() {
  const params = useParams()
  const id = params.id as string

  // Status page state
  const [page, setPage] = useState<StatusPage | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  // Form state
  const [name, setName] = useState("")
  const [slug, setSlug] = useState("")
  const [isPublic, setIsPublic] = useState(true)
  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState<string | null>(null)
  const [saveSuccess, setSaveSuccess] = useState(false)

  // Associated monitors state
  const [linkedMonitors, setLinkedMonitors] = useState<StatusPageMonitor[]>([])
  const [linkedLoading, setLinkedLoading] = useState(true)

  // Remove monitor confirmation
  const [removeTarget, setRemoveTarget] = useState<StatusPageMonitor | null>(null)
  const [removing, setRemoving] = useState(false)

  // Add monitors dialog
  const [addDialogOpen, setAddDialogOpen] = useState(false)
  const [allMonitors, setAllMonitors] = useState<ApiMonitor[]>([])
  const [monitorsLoading, setMonitorsLoading] = useState(false)
  const [selectedMonitorId, setSelectedMonitorId] = useState<string | null>(null)
  const [adding, setAdding] = useState(false)
  const [addError, setAddError] = useState<string | null>(null)

  // ── Fetch status page ──────────────────────────────────────────────────────
  const fetchPage = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const json = await apiRequest<{ status_page: StatusPage }>(`/v1/status-pages/${id}`)
      const found = json.status_page
      if (!found) {
        setError("状态页不存在")
        return
      }
      setPage(found)
      setName(found.name)
      setSlug(found.slug)
      setIsPublic(found.is_public)
    } catch (e) {
      setError(e instanceof Error ? e.message : "加载失败")
    } finally {
      setLoading(false)
    }
  }, [id])

  // ── Fetch linked monitors ──────────────────────────────────────────────────
  const fetchLinkedMonitors = useCallback(async () => {
    setLinkedLoading(true)
    try {
      const json = await apiRequest<{ monitors: StatusPageMonitor[] }>(
        `/v1/status-pages/${id}/monitors`,
      )
      setLinkedMonitors(json.monitors ?? [])
    } catch {
      // Endpoint may not exist yet — treat as empty list
      setLinkedMonitors([])
    } finally {
      setLinkedLoading(false)
    }
  }, [id])

  useEffect(() => {
    fetchPage()
    fetchLinkedMonitors()
  }, [fetchPage, fetchLinkedMonitors])

  // ── Save basic info ────────────────────────────────────────────────────────
  async function handleSave() {
    if (!name.trim() || !slug.trim()) {
      setSaveError("名称和 Slug 不能为空")
      return
    }
    setSaving(true)
    setSaveError(null)
    setSaveSuccess(false)
    try {
      await apiRequest(`/v1/status-pages/${id}`, {
        method: "PATCH",
        body: JSON.stringify({ name: name.trim(), slug: slug.trim(), is_public: isPublic }),
      })
      setSaveSuccess(true)
      setTimeout(() => setSaveSuccess(false), 3000)
      // Update local page state
      setPage((prev) => prev ? { ...prev, name: name.trim(), slug: slug.trim(), is_public: isPublic } : prev)
    } catch (e) {
      setSaveError(e instanceof Error ? e.message : "保存失败")
    } finally {
      setSaving(false)
    }
  }

  // ── Remove linked monitor ──────────────────────────────────────────────────
  async function handleRemoveMonitor() {
    if (!removeTarget) return
    setRemoving(true)
    try {
      await apiRequest(`/v1/status-pages/${id}/monitors/${removeTarget.monitor_id}`, {
        method: "DELETE",
      })
      setLinkedMonitors((prev) => prev.filter((m) => m.monitor_id !== removeTarget.monitor_id))
      setRemoveTarget(null)
    } catch (e) {
      // Show error inline — keep dialog open on error
      alert(e instanceof Error ? e.message : "删除失败")
    } finally {
      setRemoving(false)
    }
  }

  // ── Open add dialog ────────────────────────────────────────────────────────
  async function handleOpenAddDialog() {
    setAddDialogOpen(true)
    setSelectedMonitorId(null)
    setAddError(null)
    setMonitorsLoading(true)
    try {
      const json = await apiRequest<{ monitors: ApiMonitor[] }>("/v1/monitors")
      const linked = new Set(linkedMonitors.map((m) => m.monitor_id))
      setAllMonitors((json.monitors ?? []).filter((m) => !linked.has(m.id)))
    } catch (e) {
      setAddError(e instanceof Error ? e.message : "加载监控列表失败")
    } finally {
      setMonitorsLoading(false)
    }
  }

  // ── Add monitor ────────────────────────────────────────────────────────────
  async function handleAddMonitor() {
    if (!selectedMonitorId) return
    setAdding(true)
    setAddError(null)
    try {
      await apiRequest(`/v1/status-pages/${id}/monitors`, {
        method: "POST",
        body: JSON.stringify({ monitor_id: selectedMonitorId }),
      })
      // Refresh linked list and close dialog
      await fetchLinkedMonitors()
      setAddDialogOpen(false)
    } catch (e) {
      setAddError(e instanceof Error ? e.message : "添加失败")
    } finally {
      setAdding(false)
    }
  }

  // ── Render ─────────────────────────────────────────────────────────────────

  if (loading) {
    return (
      <div className="space-y-6">
        <PageSkeleton />
      </div>
    )
  }

  if (error) {
    return (
      <div className="space-y-4">
        <Link
          href="/app/status-pages"
          className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
        >
          <ArrowLeft className="h-4 w-4" />
          返回状态页列表
        </Link>
        <Alert variant="destructive">
          <AlertCircle className="h-4 w-4" />
          <AlertTitle>加载失败</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      </div>
    )
  }

  const statusColors: Record<string, string> = {
    operational: "bg-green-500/10 text-green-600 border-green-500/20",
    degraded: "bg-yellow-500/10 text-yellow-600 border-yellow-500/20",
    outage: "bg-red-500/10 text-red-600 border-red-500/20",
  }

  return (
    <div className="space-y-6">
      {/* Breadcrumb */}
      <div className="space-y-1">
        <Breadcrumb>
          <BreadcrumbList>
            <BreadcrumbItem>
              <BreadcrumbLink asChild>
                <Link href="/app/status-pages">状态页管理</Link>
              </BreadcrumbLink>
            </BreadcrumbItem>
            <BreadcrumbSeparator />
            <BreadcrumbItem>
              <BreadcrumbPage>{page?.name ?? id}</BreadcrumbPage>
            </BreadcrumbItem>
          </BreadcrumbList>
        </Breadcrumb>

        <div className="flex items-center gap-3">
          <h1 className="text-2xl font-bold tracking-tight">{page?.name}</h1>
          {page?.overall_status && (
            <Badge
              variant="outline"
              className={statusColors[page.overall_status] ?? ""}
            >
              {page.overall_status}
            </Badge>
          )}
        </div>

        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <span>
            公开地址：
            <a
              href={`/status/${page?.slug}`}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-1 text-foreground underline-offset-4 hover:underline"
            >
              /status/{page?.slug}
              <ExternalLink className="h-3 w-3" />
            </a>
          </span>
        </div>
      </div>

      {/* Basic info form */}
      <Card>
        <CardHeader>
          <CardTitle>基本信息</CardTitle>
          <CardDescription>修改状态页名称、Slug 和可见性</CardDescription>
        </CardHeader>
        <CardContent className="space-y-5">
          {saveError && (
            <Alert variant="destructive">
              <AlertCircle className="h-4 w-4" />
              <AlertDescription>{saveError}</AlertDescription>
            </Alert>
          )}
          {saveSuccess && (
            <Alert>
              <AlertDescription>保存成功</AlertDescription>
            </Alert>
          )}

          <div className="space-y-1.5">
            <Label htmlFor="sp-name">名称</Label>
            <Input
              id="sp-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="我的服务状态页"
              disabled={saving}
            />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="sp-slug">Slug</Label>
            <div className="flex items-center gap-2">
              <span className="text-sm text-muted-foreground">/status/</span>
              <Input
                id="sp-slug"
                value={slug}
                onChange={(e) =>
                  setSlug(
                    e.target.value
                      .toLowerCase()
                      .replace(/[^a-z0-9-]/g, "-")
                      .replace(/-+/g, "-")
                      .replace(/^-|-$/g, ""),
                  )
                }
                placeholder="my-service"
                disabled={saving}
                className="flex-1"
              />
            </div>
            <p className="text-xs text-muted-foreground">
              只允许小写字母、数字和连字符
            </p>
          </div>

          <div className="flex items-center justify-between rounded-md border p-3">
            <div className="space-y-0.5">
              <Label htmlFor="sp-public" className="cursor-pointer text-sm font-medium">
                公开可见
              </Label>
              <p className="text-xs text-muted-foreground">
                开启后，任何人可通过链接访问此状态页
              </p>
            </div>
            <Switch
              id="sp-public"
              checked={isPublic}
              onCheckedChange={setIsPublic}
              disabled={saving}
            />
          </div>

          <Button onClick={handleSave} disabled={saving}>
            {saving ? "保存中…" : "保存"}
          </Button>
        </CardContent>
      </Card>

      {/* Linked monitors */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0">
          <div>
            <CardTitle>关联监控</CardTitle>
            <CardDescription className="mt-1">
              此状态页显示的监控项目
            </CardDescription>
          </div>
          <Button size="sm" variant="outline" onClick={handleOpenAddDialog}>
            <Plus className="mr-1 h-4 w-4" />
            添加监控
          </Button>
        </CardHeader>
        <CardContent>
          {linkedLoading ? (
            <div className="space-y-3">
              {[1, 2].map((i) => (
                <div
                  key={i}
                  className="flex items-center justify-between rounded-md border p-3"
                >
                  <div className="space-y-1">
                    <Skeleton className="h-4 w-40" />
                    <Skeleton className="h-3 w-28" />
                  </div>
                  <Skeleton className="h-8 w-8" />
                </div>
              ))}
            </div>
          ) : linkedMonitors.length === 0 ? (
            <div className="flex flex-col items-center justify-center rounded-md border border-dashed py-10 text-center">
              <p className="text-sm text-muted-foreground">暂无关联监控</p>
              <Button
                size="sm"
                variant="ghost"
                className="mt-2"
                onClick={handleOpenAddDialog}
              >
                <Plus className="mr-1 h-4 w-4" />
                添加第一个监控
              </Button>
            </div>
          ) : (
            <div className="space-y-2">
              {linkedMonitors.map((m) => (
                <div
                  key={m.monitor_id}
                  className="flex items-center justify-between rounded-md border p-3"
                >
                  <div className="min-w-0 flex-1 space-y-0.5">
                    <p className="truncate text-sm font-medium">{m.name}</p>
                    <p className="truncate text-xs text-muted-foreground">{m.target}</p>
                  </div>
                  <div className="ml-3 flex items-center gap-2">
                    <Badge variant="outline" className="text-xs">
                      {m.type}
                    </Badge>
                    <Button
                      size="icon"
                      variant="ghost"
                      className="h-8 w-8 text-muted-foreground hover:text-destructive"
                      onClick={() => setRemoveTarget(m)}
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      {/* Remove monitor confirmation */}
      <AlertDialog
        open={!!removeTarget}
        onOpenChange={(open) => { if (!open) setRemoveTarget(null) }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>移除关联监控</AlertDialogTitle>
            <AlertDialogDescription>
              确定要从此状态页移除监控「{removeTarget?.name}」吗？此操作不会删除监控本身。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={removing}>取消</AlertDialogCancel>
            <AlertDialogAction
              onClick={handleRemoveMonitor}
              disabled={removing}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              {removing ? "移除中…" : "确认移除"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* Add monitor dialog */}
      <Dialog open={addDialogOpen} onOpenChange={setAddDialogOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>添加监控</DialogTitle>
            <DialogDescription>
              选择要关联到此状态页的监控项目
            </DialogDescription>
          </DialogHeader>

          <div className="max-h-72 overflow-y-auto">
            {monitorsLoading ? (
              <div className="space-y-2 p-1">
                {[1, 2, 3].map((i) => (
                  <div key={i} className="flex items-center gap-3 rounded-md border p-3">
                    <Skeleton className="h-4 w-4 rounded-sm" />
                    <div className="flex-1 space-y-1">
                      <Skeleton className="h-4 w-36" />
                      <Skeleton className="h-3 w-28" />
                    </div>
                  </div>
                ))}
              </div>
            ) : addError ? (
              <Alert variant="destructive" className="mx-1">
                <AlertCircle className="h-4 w-4" />
                <AlertDescription>{addError}</AlertDescription>
              </Alert>
            ) : allMonitors.length === 0 ? (
              <p className="py-6 text-center text-sm text-muted-foreground">
                所有监控均已关联，或暂无可用监控
              </p>
            ) : (
              <div className="space-y-1 p-1">
                {allMonitors.map((m) => {
                  const selected = selectedMonitorId === m.id
                  return (
                    <button
                      key={m.id}
                      type="button"
                      onClick={() => setSelectedMonitorId(selected ? null : m.id)}
                      className={`w-full rounded-md border p-3 text-left transition-colors ${
                        selected
                          ? "border-primary bg-primary/5"
                          : "hover:bg-accent"
                      }`}
                    >
                      <p className="text-sm font-medium">{m.name}</p>
                      <p className="text-xs text-muted-foreground">{m.target}</p>
                    </button>
                  )
                })}
              </div>
            )}
          </div>

          {/* Show add-action error below the list (not the fetch error shown inside scroll area) */}
          {addError && !monitorsLoading && allMonitors.length > 0 && (
            <Alert variant="destructive">
              <AlertCircle className="h-4 w-4" />
              <AlertDescription>{addError}</AlertDescription>
            </Alert>
          )}

          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setAddDialogOpen(false)}
              disabled={adding}
            >
              取消
            </Button>
            <Button
              onClick={handleAddMonitor}
              disabled={!selectedMonitorId || adding}
            >
              {adding ? "添加中…" : "添加"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
