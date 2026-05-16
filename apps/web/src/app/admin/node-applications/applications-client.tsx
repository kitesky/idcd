"use client"

import { useState, useTransition } from "react"
import { toast } from "sonner"
import { useTranslations } from "next-intl"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { reviewApplicationAction } from "./actions"

export interface NodeApplication {
  id: string
  user_id: string
  hostname: string
  ip_address: string
  country: string
  city: string
  isp: string
  status: "pending" | "probation" | "approved" | "rejected" | "active"
  created_at: string
  updated_at: string
}

function formatTime(iso: string) {
  try {
    return new Date(iso).toLocaleString(undefined, {
      month: "2-digit", day: "2-digit",
      hour: "2-digit", minute: "2-digit",
    })
  } catch { return iso }
}

interface ReviewDialogProps {
  app: NodeApplication
  onDone: (id: string, newStatus: NodeApplication["status"]) => void
}

function ReviewDialog({ app, onDone }: ReviewDialogProps) {
  const t = useTranslations("admin")
  const [open, setOpen] = useState(false)
  const [action, setAction] = useState<"approve" | "reject">("approve")
  const [note, setNote] = useState("")
  const [pending, startTransition] = useTransition()

  function statusBadge(status: NodeApplication["status"]) {
    switch (status) {
      case "pending":   return <Badge variant="outline">{t("nodeApplications.status.pending")}</Badge>
      case "probation": return <Badge variant="secondary">{t("nodeApplications.status.probation")}</Badge>
      case "approved":  return <Badge variant="default">{t("nodeApplications.status.approved")}</Badge>
      case "active":    return <Badge variant="success">{t("nodeApplications.status.active")}</Badge>
      case "rejected":  return <Badge variant="destructive">{t("nodeApplications.status.rejected")}</Badge>
    }
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    startTransition(async () => {
      const res = await reviewApplicationAction(app.id, action, note.trim())
      if (!res.ok) {
        const msg = res.messageKey ? t(res.messageKey) : res.message
        toast.error(msg ?? t("nodeApplications.errors.reviewFailed"))
      } else {
        toast.success(action === "approve" ? t("nodeApplications.toast.approved") : t("nodeApplications.toast.rejected"))
        onDone(app.id, action === "approve" ? "probation" : "rejected")
        setOpen(false)
        setNote("")
      }
    })
  }

  return (
    <>
      <Button variant="outline" size="sm" onClick={() => setOpen(true)}>{t("nodeApplications.toast.review")}</Button>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t("nodeApplications.reviewDialog")}</DialogTitle>
          </DialogHeader>
          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="rounded-md border px-4 py-3 text-sm space-y-1">
              <p><span className="text-muted-foreground">{t("nodeApplications.applicationDetail.hostname")}</span>{app.hostname}</p>
              <p><span className="text-muted-foreground">{t("nodeApplications.applicationDetail.ip")}</span><code className="font-mono">{app.ip_address}</code></p>
              <p><span className="text-muted-foreground">{t("nodeApplications.applicationDetail.location")}</span>{app.city}, {app.country}</p>
              <p><span className="text-muted-foreground">{t("nodeApplications.applicationDetail.isp")}</span>{app.isp}</p>
            </div>
            <div className="space-y-1.5">
              <Label>{t("nodeApplications.reviewAction")}</Label>
              <Select value={action} onValueChange={(v) => setAction(v as "approve" | "reject")}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="approve">{t("nodeApplications.approveOption")}</SelectItem>
                  <SelectItem value="reject">{t("nodeApplications.rejectOption")}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="review-note">{t("nodeApplications.reviewNote")}</Label>
              <Input
                id="review-note"
                value={note}
                onChange={(e) => setNote(e.target.value)}
                placeholder={t("nodeApplications.reviewNotePlaceholder")}
              />
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setOpen(false)} disabled={pending}>
                {t("nodeApplications.cancel")}
              </Button>
              <Button type="submit" disabled={pending} variant={action === "reject" ? "destructive" : "default"}>
                {pending ? t("nodeApplications.submit") : action === "approve" ? t("nodeApplications.approve") : t("nodeApplications.reject")}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </>
  )
}

export function ApplicationsClient({ initialApps }: { initialApps: NodeApplication[] }) {
  const t = useTranslations("admin")
  const [apps, setApps] = useState(initialApps)
  const [statusFilter, setStatusFilter] = useState<string>("all")

  function statusBadge(status: NodeApplication["status"]) {
    switch (status) {
      case "pending":   return <Badge variant="outline">{t("nodeApplications.status.pending")}</Badge>
      case "probation": return <Badge variant="secondary">{t("nodeApplications.status.probation")}</Badge>
      case "approved":  return <Badge variant="default">{t("nodeApplications.status.approved")}</Badge>
      case "active":    return <Badge variant="success">{t("nodeApplications.status.active")}</Badge>
      case "rejected":  return <Badge variant="destructive">{t("nodeApplications.status.rejected")}</Badge>
    }
  }

  function handleReviewed(id: string, newStatus: NodeApplication["status"]) {
    setApps(prev => prev.map(a => a.id === id ? { ...a, status: newStatus } : a))
  }

  const filtered = statusFilter === "all" ? apps : apps.filter(a => a.status === statusFilter)
  const pendingCount = apps.filter(a => a.status === "pending").length

  return (
    <div className="space-y-6">
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
        {[
          { label: t("nodeApplications.pendingCount"), count: apps.filter(a => a.status === "pending").length, color: "text-yellow-500" },
          { label: t("nodeApplications.probation"), count: apps.filter(a => a.status === "probation").length, color: "text-blue-500" },
          { label: t("nodeApplications.activated"), count: apps.filter(a => a.status === "active").length, color: "text-green-500" },
          { label: t("nodeApplications.rejected"), count: apps.filter(a => a.status === "rejected").length, color: "text-destructive" },
        ].map(({ label, count, color }) => (
          <Card key={label}>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium text-muted-foreground">{label}</CardTitle>
            </CardHeader>
            <CardContent><p className={`text-2xl font-bold ${color}`}>{count}</p></CardContent>
          </Card>
        ))}
      </div>

      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle>
            {t("nodeApplications.listTitle")}
            {pendingCount > 0 && <Badge variant="destructive" className="ml-2">{pendingCount}</Badge>}
          </CardTitle>
          <Select value={statusFilter} onValueChange={setStatusFilter}>
            <SelectTrigger className="w-32">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t("nodeApplications.filter.all")}</SelectItem>
              <SelectItem value="pending">{t("nodeApplications.status.pending")}</SelectItem>
              <SelectItem value="probation">{t("nodeApplications.status.probation")}</SelectItem>
              <SelectItem value="active">{t("nodeApplications.status.active")}</SelectItem>
              <SelectItem value="rejected">{t("nodeApplications.status.rejected")}</SelectItem>
            </SelectContent>
          </Select>
        </CardHeader>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("nodeApplications.table.hostname")}</TableHead>
                <TableHead>{t("nodeApplications.table.ip")}</TableHead>
                <TableHead>{t("nodeApplications.table.location")}</TableHead>
                <TableHead>{t("nodeApplications.table.isp")}</TableHead>
                <TableHead>{t("nodeApplications.table.status")}</TableHead>
                <TableHead>{t("nodeApplications.table.appliedAt")}</TableHead>
                <TableHead>{t("nodeApplications.table.actions")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {filtered.length === 0 && (
                <TableRow>
                  <TableCell colSpan={7} className="py-8 text-center text-muted-foreground">
                    {t("nodeApplications.noData")}
                  </TableCell>
                </TableRow>
              )}
              {filtered.map(app => (
                <TableRow key={app.id}>
                  <TableCell className="font-mono text-xs">{app.hostname}</TableCell>
                  <TableCell className="font-mono text-xs">{app.ip_address}</TableCell>
                  <TableCell className="text-sm text-muted-foreground">{app.city}, {app.country}</TableCell>
                  <TableCell className="text-sm text-muted-foreground">{app.isp}</TableCell>
                  <TableCell>{statusBadge(app.status)}</TableCell>
                  <TableCell className="text-xs text-muted-foreground">{formatTime(app.created_at)}</TableCell>
                  <TableCell>
                    {(app.status === "pending" || app.status === "probation") && (
                      <ReviewDialog app={app} onDone={handleReviewed} />
                    )}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  )
}
