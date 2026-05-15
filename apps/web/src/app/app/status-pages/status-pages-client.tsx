"use client"

import { useState, useEffect, useCallback } from "react"
import { ExternalLink, Plus, AlertCircle } from "lucide-react"
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Separator } from "@/components/ui/separator"
import { Skeleton } from "@/components/ui/skeleton"
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogAction,
  AlertDialogCancel,
} from "@/components/ui/alert-dialog"
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { apiRequest } from "@/lib/api"

interface StatusPage {
  id: string
  name: string
  slug: string
  is_public: boolean
  overall_status: string
  created_at: string
}

function StatusPageSkeleton() {
  return (
    <div className="space-y-3" data-testid="status-pages-skeleton">
      {[1, 2, 3].map((i) => (
        <Card key={i}>
          <CardContent className="flex items-center justify-between py-4 px-5">
            <div className="space-y-2">
              <Skeleton className="h-4 w-48" />
              <Skeleton className="h-3 w-32" />
            </div>
            <Skeleton className="h-4 w-12" />
          </CardContent>
        </Card>
      ))}
    </div>
  )
}

interface CreateSheetContentProps {
  onClose: () => void
  onCreated: () => void
}

function CreateSheetContent({ onClose, onCreated }: CreateSheetContentProps) {
  const [name, setName] = useState("")
  const [slug, setSlug] = useState("")
  const [desc, setDesc] = useState("")
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  function handleNameChange(val: string) {
    setName(val)
    setSlug(
      val
        .toLowerCase()
        .replace(/\s+/g, "-")
        .replace(/[^a-z0-9-]/g, "")
    )
  }

  async function handleCreate() {
    if (!name.trim() || !slug.trim()) return
    setSubmitting(true)
    setError(null)
    try {
      await apiRequest("/v1/status-pages", {
        method: "POST",
        body: JSON.stringify({ name: name.trim(), slug: slug.trim(), is_public: true }),
      })
      onCreated()
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : "创建失败，请重试")
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="space-y-4 mt-4">
      {error && (
        <Alert variant="destructive" data-testid="create-sp-error">
          <AlertCircle className="h-4 w-4" />
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}
      <div className="space-y-1.5">
        <Label htmlFor="sp-name">页面名称</Label>
        <Input
          id="sp-name"
          placeholder="例：acme.com 服务状态"
          value={name}
          onChange={(e) => handleNameChange(e.target.value)}
          data-testid="sp-name-input"
        />
      </div>
      <div className="space-y-1.5">
        <Label htmlFor="sp-slug">Slug（访问路径）</Label>
        <div className="flex items-center gap-2">
          <span className="text-sm text-muted-foreground shrink-0">
            .status.idcd.com/
          </span>
          <Input
            id="sp-slug"
            placeholder="acme"
            value={slug}
            onChange={(e) => setSlug(e.target.value)}
            data-testid="sp-slug-input"
          />
        </div>
      </div>
      <div className="space-y-1.5">
        <Label htmlFor="sp-desc">描述（可选）</Label>
        <Textarea
          id="sp-desc"
          placeholder="简短说明该状态页用途..."
          value={desc}
          onChange={(e) => setDesc(e.target.value)}
          rows={3}
          data-testid="sp-desc-input"
        />
      </div>
      <Separator />
      <div className="flex gap-2">
        <Button
          className="flex-1"
          onClick={handleCreate}
          disabled={submitting || !name.trim() || !slug.trim()}
          data-testid="create-sp-button"
        >
          {submitting ? "创建中..." : "创建状态页"}
        </Button>
        <Button variant="outline" onClick={onClose} disabled={submitting} data-testid="cancel-sp-button">
          取消
        </Button>
      </div>
    </div>
  )
}

export function StatusPagesClient() {
  const [statusPages, setStatusPages] = useState<StatusPage[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [showUpgradeDialog, setShowUpgradeDialog] = useState(false)
  const [showCreateSheet, setShowCreateSheet] = useState(false)
  const [deletingId, setDeletingId] = useState<string | null>(null)
  const [confirmDeleteId, setConfirmDeleteId] = useState<string | null>(null)
  const [isFreePlan, setIsFreePlan] = useState(false)
  const [quotaLoading, setQuotaLoading] = useState(true)

  const fetchStatusPages = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const res = await apiRequest<{ data: { status_pages: StatusPage[] } }>("/v1/status-pages")
      setStatusPages(res.data.status_pages ?? [])
    } catch (err) {
      setError(err instanceof Error ? err.message : "加载失败，请刷新重试")
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchStatusPages()
  }, [fetchStatusPages])

  useEffect(() => {
    apiRequest<{ data: { plan: string } }>("/v1/account/quota")
      .then((res) => {
        setIsFreePlan(res.data.plan === "free")
      })
      .catch(() => {
        setIsFreePlan(false)
      })
      .finally(() => {
        setQuotaLoading(false)
      })
  }, [])

  function handleNewPage() {
    if (isFreePlan) {
      setShowUpgradeDialog(true)
    } else {
      setShowCreateSheet(true)
    }
  }

  async function handleDelete(id: string) {
    setDeletingId(id)
    try {
      await apiRequest(`/v1/status-pages/${id}`, { method: "DELETE" })
      setStatusPages((prev) => prev.filter((sp) => sp.id !== id))
    } catch (err) {
      setError(err instanceof Error ? err.message : "删除失败，请重试")
    } finally {
      setDeletingId(null)
      setConfirmDeleteId(null)
    }
  }

  return (
    <div className="space-y-6" data-testid="status-pages-page">
      {!quotaLoading && isFreePlan && (
        <Alert data-testid="free-plan-notice">
          <AlertTitle className="flex items-center gap-2">
            Free 档限制
            <Badge variant="warning" className="text-xs">
              升级 Pro 解锁
            </Badge>
          </AlertTitle>
          <AlertDescription>
            Free 档不支持创建状态页。升级到 Pro 可获得最多 3 个状态页，Team 可获得 10 个。
          </AlertDescription>
        </Alert>
      )}

      {error && (
        <Alert variant="destructive" data-testid="sp-error-alert">
          <AlertCircle className="h-4 w-4" />
          <AlertTitle>错误</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold">状态页列表</h2>
        <Button onClick={handleNewPage} disabled={quotaLoading} data-testid="new-page-button">
          <Plus className="mr-2 h-4 w-4" />
          新建状态页
        </Button>
      </div>

      {loading ? (
        <StatusPageSkeleton />
      ) : statusPages.length === 0 ? (
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-12 text-center" data-testid="sp-empty-state">
            <p className="text-sm text-muted-foreground">暂无状态页</p>
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-3" data-testid="status-pages-list">
          {statusPages.map((sp) => (
            <Card key={sp.id} data-testid={`status-page-card-${sp.id}`}>
              <CardContent className="flex items-center justify-between py-4 px-5">
                <div className="space-y-1">
                  <div className="flex items-center gap-2">
                    <span className="font-medium text-sm">{sp.name}</span>
                  </div>
                  <div className="flex items-center gap-3 text-xs text-muted-foreground">
                    <span className="font-mono">{sp.slug}</span>
                  </div>
                </div>
                <div className="flex items-center gap-3">
                  <a
                    href={`https://${sp.slug}.status.idcd.com`}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="inline-flex items-center gap-1 text-xs text-primary hover:underline underline-offset-4"
                    data-testid={`status-page-link-${sp.id}`}
                  >
                    访问
                    <ExternalLink className="h-3 w-3" />
                  </a>
                  <Button
                    variant="ghost"
                    size="sm"
                    className="text-destructive hover:text-destructive"
                    onClick={() => setConfirmDeleteId(sp.id)}
                    disabled={deletingId === sp.id}
                    data-testid={`delete-sp-btn-${sp.id}`}
                  >
                    {deletingId === sp.id ? "删除中..." : "删除"}
                  </Button>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      {/* Upgrade dialog */}
      <AlertDialog open={showUpgradeDialog} onOpenChange={setShowUpgradeDialog}>
        <AlertDialogContent data-testid="upgrade-dialog">
          <AlertDialogHeader>
            <AlertDialogTitle>升级解锁状态页</AlertDialogTitle>
            <AlertDialogDescription>
              Free 档不支持创建状态页。升级到 Pro 可创建最多 3 个状态页，支持自定义品牌与监控绑定。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel data-testid="upgrade-cancel-button">稍后再说</AlertDialogCancel>
            <AlertDialogAction data-testid="upgrade-confirm-button">升级到 Pro</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* Delete confirm dialog */}
      <AlertDialog open={!!confirmDeleteId} onOpenChange={(open) => !open && setConfirmDeleteId(null)}>
        <AlertDialogContent data-testid="delete-confirm-dialog">
          <AlertDialogHeader>
            <AlertDialogTitle>确认删除</AlertDialogTitle>
            <AlertDialogDescription>
              此操作不可恢复，确定要删除该状态页吗？
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel data-testid="delete-cancel-button">取消</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => confirmDeleteId && handleDelete(confirmDeleteId)}
              data-testid="delete-confirm-button"
            >
              确认删除
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* Create sheet */}
      <Sheet open={showCreateSheet} onOpenChange={(open) => !open && setShowCreateSheet(false)}>
        <SheetContent data-testid="create-sheet">
          <SheetHeader>
            <SheetTitle>新建状态页</SheetTitle>
          </SheetHeader>
          <CreateSheetContent
            onClose={() => setShowCreateSheet(false)}
            onCreated={fetchStatusPages}
          />
        </SheetContent>
      </Sheet>
    </div>
  )
}
