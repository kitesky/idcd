"use client"

import { useState, useEffect, useCallback } from "react"
import { ExternalLink, Plus, AlertCircle } from "lucide-react"
import { useTranslations } from "next-intl"
import {
  Card,
  CardContent,
} from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
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
  SheetFooter,
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
  const t = useTranslations("status.statusPages")
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
      setError(err instanceof Error ? err.message : t("createSheet.createFailed"))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <>
      <div className="flex-1 overflow-y-auto px-6 py-6 space-y-4">
        {error && (
          <Alert variant="destructive" data-testid="create-sp-error">
            <AlertCircle className="h-4 w-4" />
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        )}
        <div className="space-y-1.5">
          <Label htmlFor="sp-name">{t("createSheet.name")}</Label>
          <Input
            id="sp-name"
            placeholder={t("createSheet.namePlaceholder")}
            value={name}
            onChange={(e) => handleNameChange(e.target.value)}
            data-testid="sp-name-input"
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="sp-slug">{t("createSheet.slug")}</Label>
          <div className="flex items-center gap-2">
            <span className="text-sm text-muted-foreground shrink-0">
              .status.idcd.com/
            </span>
            <Input
              id="sp-slug"
              placeholder={t("createSheet.slugPlaceholder")}
              value={slug}
              onChange={(e) => setSlug(e.target.value)}
              data-testid="sp-slug-input"
            />
          </div>
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="sp-desc">{t("createSheet.desc")}</Label>
          <Textarea
            id="sp-desc"
            placeholder={t("createSheet.descPlaceholder")}
            value={desc}
            onChange={(e) => setDesc(e.target.value)}
            rows={3}
            data-testid="sp-desc-input"
          />
        </div>
      </div>
      <SheetFooter className="shrink-0 border-t px-6 py-4 flex-row gap-3 mt-0">
        <Button
          className="flex-1"
          onClick={handleCreate}
          disabled={submitting || !name.trim() || !slug.trim()}
          data-testid="create-sp-button"
        >
          {submitting ? t("createSheet.creating") : t("createSheet.create")}
        </Button>
        <Button variant="outline" onClick={onClose} disabled={submitting} data-testid="cancel-sp-button">
          {t("createSheet.cancel")}
        </Button>
      </SheetFooter>
    </>
  )
}

export function StatusPagesClient() {
  const t = useTranslations("status.statusPages")
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
      setError(err instanceof Error ? err.message : t("loadFailed"))
    } finally {
      setLoading(false)
    }
  }, [t])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- fetchStatusPages 内部 await 后 setState
    void fetchStatusPages()
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
      setError(err instanceof Error ? err.message : t("createSheet.createFailed"))
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
            {t("freePlan.title")}
            <Badge variant="warning" className="text-xs">
              {t("freePlan.upgrade")}
            </Badge>
          </AlertTitle>
          <AlertDescription>
            {t("freePlan.desc")}
          </AlertDescription>
        </Alert>
      )}

      {error && (
        <Alert variant="destructive" data-testid="sp-error-alert">
          <AlertCircle className="h-4 w-4" />
          <AlertTitle>{t("error")}</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold">{t("list")}</h2>
        <Button onClick={handleNewPage} disabled={quotaLoading} data-testid="new-page-button">
          <Plus className="mr-2 h-4 w-4" />
          {t("create")}
        </Button>
      </div>

      {loading ? (
        <StatusPageSkeleton />
      ) : statusPages.length === 0 ? (
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-12 text-center" data-testid="sp-empty-state">
            <p className="text-sm text-muted-foreground">{t("empty")}</p>
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
                    {t("visit")}
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
                    {deletingId === sp.id ? t("deleting") : t("delete")}
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
            <AlertDialogTitle>{t("upgradeDialog.title")}</AlertDialogTitle>
            <AlertDialogDescription>
              {t("upgradeDialog.desc")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel data-testid="upgrade-cancel-button">{t("upgradeDialog.later")}</AlertDialogCancel>
            <AlertDialogAction data-testid="upgrade-confirm-button">{t("upgradeDialog.confirm")}</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* Delete confirm dialog */}
      <AlertDialog open={!!confirmDeleteId} onOpenChange={(open) => !open && setConfirmDeleteId(null)}>
        <AlertDialogContent data-testid="delete-confirm-dialog">
          <AlertDialogHeader>
            <AlertDialogTitle>{t("deleteDialog.title")}</AlertDialogTitle>
            <AlertDialogDescription>
              {t("deleteDialog.desc")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel data-testid="delete-cancel-button">{t("deleteDialog.cancel")}</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => confirmDeleteId && handleDelete(confirmDeleteId)}
              data-testid="delete-confirm-button"
            >
              {t("deleteDialog.confirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* Create sheet */}
      <Sheet open={showCreateSheet} onOpenChange={(open) => !open && setShowCreateSheet(false)}>
        <SheetContent className="flex flex-col gap-0 p-0 overflow-hidden" data-testid="create-sheet">
          <SheetHeader className="shrink-0 border-b px-6 py-4">
            <SheetTitle>{t("createSheet.title")}</SheetTitle>
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
