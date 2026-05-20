"use client"

import { useState, useEffect } from "react"
import Link from "next/link"
import { useParams } from "next/navigation"
import { useTranslations } from "next-intl"
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
import { toast } from "sonner"

// ── Typed translator alias ───────────────────────────────────────────────────

type StatusPageDetailT = ReturnType<typeof useTranslations<"status.statusPages.detail">>

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
  const t = useTranslations("status.statusPages.detail")
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

  // ── Fetch status page + linked monitors on mount / id change ───────────────
  useEffect(() => {
    let cancelled = false
    // eslint-disable-next-line react-hooks/set-state-in-effect -- 异步 fetch；id 变化触发
    setLoading(true)
    setError(null)
    apiRequest<{ data: { status_page: StatusPage } }>(`/v1/status-pages/${id}`)
      .then((json) => {
        if (cancelled) return
        const found = json?.data?.status_page
        if (!found) {
          setError(t("notFound"))
          return
        }
        setPage(found)
        setName(found.name)
        setSlug(found.slug)
        setIsPublic(found.is_public)
      })
      .catch((e) => {
        if (cancelled) return
        setError(e instanceof Error ? e.message : t("loadFailed"))
      })
      .finally(() => {
        if (cancelled) return
        setLoading(false)
      })
    return () => {
      cancelled = true
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps -- t 来自 i18n hook，引用稳定但 lint 不识别
  }, [id])

  useEffect(() => {
    let cancelled = false
    // eslint-disable-next-line react-hooks/set-state-in-effect -- 异步 fetch；id 变化触发
    setLinkedLoading(true)
    apiRequest<{ data: { monitors: StatusPageMonitor[] } }>(
      `/v1/status-pages/${id}/monitors`,
    )
      .then((json) => {
        if (cancelled) return
        setLinkedMonitors(json?.data?.monitors ?? [])
      })
      .catch(() => {
        if (cancelled) return
        // Endpoint may not exist yet — treat as empty list
        setLinkedMonitors([])
      })
      .finally(() => {
        if (cancelled) return
        setLinkedLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [id])

  // Refetch linked monitors after add — separate helper, not in deps
  async function refetchLinkedMonitors() {
    setLinkedLoading(true)
    try {
      const json = await apiRequest<{ data: { monitors: StatusPageMonitor[] } }>(
        `/v1/status-pages/${id}/monitors`,
      )
      setLinkedMonitors(json?.data?.monitors ?? [])
    } catch {
      setLinkedMonitors([])
    } finally {
      setLinkedLoading(false)
    }
  }

  // ── Save basic info ────────────────────────────────────────────────────────
  async function handleSave() {
    if (!name.trim() || !slug.trim()) {
      setSaveError(t("nameSlugRequired"))
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
      setSaveError(e instanceof Error ? e.message : t("saveFailed"))
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
      toast.error(e instanceof Error ? e.message : t("saveFailed"))
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
      const json = await apiRequest<{ data: { items: ApiMonitor[] } }>("/v1/monitors")
      const linked = new Set(linkedMonitors.map((m) => m.monitor_id))
      setAllMonitors((json.data.items ?? []).filter((m) => !linked.has(m.id)))
    } catch (e) {
      setAddError(e instanceof Error ? e.message : t("addMonitorDialog.loading"))
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
      await refetchLinkedMonitors()
      setAddDialogOpen(false)
    } catch (e) {
      setAddError(e instanceof Error ? e.message : t("saveFailed"))
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
          {t("backToList")}
        </Link>
        <Alert variant="destructive">
          <AlertCircle className="h-4 w-4" />
          <AlertTitle>{t("loadFailed")}</AlertTitle>
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
                <Link href="/app/status-pages">{t("backToList")}</Link>
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
            {t("publicUrl")}
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
          <CardTitle>{t("basicInfo")}</CardTitle>
          <CardDescription>{t("basicInfoDesc")}</CardDescription>
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
              <AlertDescription>{t("saveSuccess")}</AlertDescription>
            </Alert>
          )}

          <div className="space-y-1.5">
            <Label htmlFor="sp-name">{t("name")}</Label>
            <Input
              id="sp-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder={t("namePlaceholder")}
              disabled={saving}
            />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="sp-slug">{t("slug")}</Label>
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
            <p className="text-xs text-muted-foreground">{t("slugHint")}</p>
          </div>

          <div className="flex items-center justify-between rounded-md border p-3">
            <div className="space-y-0.5">
              <Label htmlFor="sp-public" className="cursor-pointer text-sm font-medium">
                {t("publicVisible")}
              </Label>
              <p className="text-xs text-muted-foreground">{t("publicVisibleDesc")}</p>
            </div>
            <Switch
              id="sp-public"
              checked={isPublic}
              onCheckedChange={setIsPublic}
              disabled={saving}
            />
          </div>

          <Button onClick={handleSave} disabled={saving}>
            {saving ? t("saving") : t("save")}
          </Button>
        </CardContent>
      </Card>

      {/* Linked monitors */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0">
          <div>
            <CardTitle>{t("linkedMonitors")}</CardTitle>
            <CardDescription className="mt-1">{t("linkedMonitorsDesc")}</CardDescription>
          </div>
          <Button size="sm" variant="outline" onClick={handleOpenAddDialog}>
            <Plus className="mr-1 h-4 w-4" />
            {t("addMonitor")}
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
              <p className="text-sm text-muted-foreground">{t("noLinkedMonitors")}</p>
              <Button
                size="sm"
                variant="ghost"
                className="mt-2"
                onClick={handleOpenAddDialog}
              >
                <Plus className="mr-1 h-4 w-4" />
                {t("addFirstMonitor")}
              </Button>
            </div>
          ) : (
            <MonitorList
              monitors={linkedMonitors}
              onRemove={(m) => setRemoveTarget(m)}
            />
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
            <AlertDialogTitle>{t("removeMonitor")}</AlertDialogTitle>
            <AlertDialogDescription>
              {t("removeMonitorDesc", { name: removeTarget?.name ?? "" })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={removing}>
              {t("addMonitorDialog.cancel")}
            </AlertDialogCancel>
            <AlertDialogAction
              onClick={handleRemoveMonitor}
              disabled={removing}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              {removing ? t("removing") : t("confirmRemove")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* Add monitor dialog */}
      <AddMonitorDialog
        open={addDialogOpen}
        onOpenChange={setAddDialogOpen}
        monitorsLoading={monitorsLoading}
        addError={addError}
        allMonitors={allMonitors}
        selectedMonitorId={selectedMonitorId}
        onSelect={setSelectedMonitorId}
        adding={adding}
        onCancel={() => setAddDialogOpen(false)}
        onConfirm={handleAddMonitor}
        t={t}
      />
    </div>
  )
}

// ── Linked monitor list (extracted to keep main component readable) ──────────

interface MonitorListProps {
  monitors: StatusPageMonitor[]
  onRemove: (m: StatusPageMonitor) => void
}

function MonitorList({ monitors, onRemove }: MonitorListProps) {
  return (
    <div className="space-y-2">
      {monitors.map((m) => (
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
              onClick={() => onRemove(m)}
            >
              <Trash2 className="h-4 w-4" />
            </Button>
          </div>
        </div>
      ))}
    </div>
  )
}

// ── Add monitor dialog (extracted, takes typed t) ────────────────────────────

interface AddMonitorDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  monitorsLoading: boolean
  addError: string | null
  allMonitors: ApiMonitor[]
  selectedMonitorId: string | null
  onSelect: (id: string | null) => void
  adding: boolean
  onCancel: () => void
  onConfirm: () => void
  t: StatusPageDetailT
}

function AddMonitorDialog({
  open,
  onOpenChange,
  monitorsLoading,
  addError,
  allMonitors,
  selectedMonitorId,
  onSelect,
  adding,
  onCancel,
  onConfirm,
  t,
}: AddMonitorDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>{t("addMonitorDialog.title")}</DialogTitle>
          <DialogDescription>{t("addMonitorDialog.desc")}</DialogDescription>
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
              {t("addMonitorDialog.noAvailable")}
            </p>
          ) : (
            <div className="space-y-1 p-1">
              {allMonitors.map((m) => {
                const selected = selectedMonitorId === m.id
                return (
                  <button
                    key={m.id}
                    type="button"
                    onClick={() => onSelect(selected ? null : m.id)}
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
            onClick={onCancel}
            disabled={adding}
          >
            {t("addMonitorDialog.cancel")}
          </Button>
          <Button
            onClick={onConfirm}
            disabled={!selectedMonitorId || adding}
          >
            {adding ? t("addMonitorDialog.adding") : t("addMonitorDialog.add")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
