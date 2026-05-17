"use client"

import { useCallback, useState } from "react"
import { useTranslations } from "next-intl"
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
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { listOrders, forceFail, type AdminCertOrder, type AdminOrderFilter } from "../admin-cert-api"

const STATUS_OPTIONS = [
  "draft",
  "validating",
  "awaiting_org_validation",
  "issuing",
  "issued",
  "failed",
  "revoking",
  "revoked",
] as const

const CA_OPTIONS = ["lets-encrypt", "zerossl", "buypass"] as const

function statusVariant(status: string): "default" | "secondary" | "destructive" | "outline" {
  switch (status) {
    case "issued":
      return "default"
    case "failed":
    case "revoked":
      return "destructive"
    case "validating":
    case "issuing":
    case "revoking":
      return "secondary"
    default:
      return "outline"
  }
}

function formatDate(iso: string) {
  try {
    return new Date(iso).toLocaleString(undefined, {
      month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit",
    })
  } catch {
    return iso
  }
}

export function OrdersAdminClient({ initialOrders }: { initialOrders: AdminCertOrder[] }) {
  const t = useTranslations("admin")
  const [orders, setOrders] = useState(initialOrders)
  const [loading, setLoading] = useState(false)
  const [filter, setFilter] = useState<{ status: string; ca: string; account_id: string }>({
    status: "",
    ca: "",
    account_id: "",
  })

  const [forceFailDialog, setForceFailDialog] = useState<{ orderId: number; open: boolean }>({
    orderId: 0, open: false,
  })
  const [reason, setReason] = useState("")
  const [processing, setProcessing] = useState(false)
  const [toast, setToast] = useState<{ message: string; ok: boolean } | null>(null)

  const refresh = useCallback(async (override?: AdminOrderFilter) => {
    setLoading(true)
    try {
      const merged: AdminOrderFilter = {
        status: override?.status ?? filter.status,
        ca: override?.ca ?? filter.ca,
        account_id: override?.account_id ?? filter.account_id,
        limit: 50,
      }
      const data = await listOrders(merged)
      if (data) setOrders(data.orders)
    } finally {
      setLoading(false)
    }
  }, [filter])

  const handleForceFail = useCallback(async () => {
    setProcessing(true)
    try {
      const res = await forceFail(forceFailDialog.orderId, reason || "admin force-fail")
      if (!res.ok) {
        setToast({ message: res.message, ok: false })
        return
      }
      setToast({ message: `#${forceFailDialog.orderId} → failed`, ok: true })
      setForceFailDialog({ orderId: 0, open: false })
      setReason("")
      await refresh()
    } finally {
      setProcessing(false)
    }
  }, [forceFailDialog.orderId, reason, refresh])

  const statusLabel = (s: string): string => {
    // Try the per-status translation; fall back to the raw status string
    // if next-intl falls through to the missing-key behaviour.
    try {
      return t(`cert.orders.status.${s}` as Parameters<typeof t>[0])
    } catch {
      return s
    }
  }

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>{t("cert.orders.filterStatus")} / {t("cert.orders.filterCa")} / {t("cert.orders.filterAccountId")}</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex flex-wrap items-end gap-3">
            <div className="flex flex-col gap-1">
              <Label htmlFor="filter-status">{t("cert.orders.filterStatus")}</Label>
              <Select
                value={filter.status || "__all"}
                onValueChange={(v) => setFilter((f) => ({ ...f, status: v === "__all" ? "" : v }))}
              >
                <SelectTrigger id="filter-status" className="w-48">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="__all">{t("cert.orders.filterAll")}</SelectItem>
                  {STATUS_OPTIONS.map((s) => (
                    <SelectItem key={s} value={s}>{statusLabel(s)}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="flex flex-col gap-1">
              <Label htmlFor="filter-ca">{t("cert.orders.filterCa")}</Label>
              <Select
                value={filter.ca || "__all"}
                onValueChange={(v) => setFilter((f) => ({ ...f, ca: v === "__all" ? "" : v }))}
              >
                <SelectTrigger id="filter-ca" className="w-44">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="__all">{t("cert.orders.filterAll")}</SelectItem>
                  {CA_OPTIONS.map((c) => (
                    <SelectItem key={c} value={c}>{c}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="flex flex-col gap-1">
              <Label htmlFor="filter-account">{t("cert.orders.filterAccountId")}</Label>
              <Input
                id="filter-account"
                value={filter.account_id}
                onChange={(e) => setFilter((f) => ({ ...f, account_id: e.target.value }))}
                placeholder="42"
                className="w-32"
                inputMode="numeric"
              />
            </div>

            <Button variant="default" onClick={() => void refresh()} disabled={loading}>
              {loading ? t("cert.orders.loading") : t("cert.orders.apply")}
            </Button>
          </div>
          {toast && (
            <p className={`mt-3 text-sm ${toast.ok ? "text-green-600" : "text-destructive"}`}>
              {toast.message}
            </p>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("cert.orders.title")}</CardTitle>
        </CardHeader>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("cert.orders.table.id")}</TableHead>
                <TableHead>{t("cert.orders.table.accountId")}</TableHead>
                <TableHead>{t("cert.orders.table.ca")}</TableHead>
                <TableHead>{t("cert.orders.table.status")}</TableHead>
                <TableHead>{t("cert.orders.table.sans")}</TableHead>
                <TableHead>{t("cert.orders.table.retry")}</TableHead>
                <TableHead>{t("cert.orders.table.createdAt")}</TableHead>
                <TableHead>{t("cert.orders.table.actions")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {orders.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={8} className="py-8 text-center text-muted-foreground">
                    {t("cert.orders.noData")}
                  </TableCell>
                </TableRow>
              ) : orders.map((o) => (
                <TableRow key={o.id}>
                  <TableCell className="font-mono text-xs">{o.id}</TableCell>
                  <TableCell className="font-mono text-xs">{o.account_id}</TableCell>
                  <TableCell className="font-mono text-xs">{o.ca}</TableCell>
                  <TableCell>
                    <Badge variant={statusVariant(o.status)}>{statusLabel(o.status)}</Badge>
                  </TableCell>
                  <TableCell className="max-w-xs truncate font-mono text-xs" title={o.sans.join(", ")}>
                    {o.sans.join(", ")}
                  </TableCell>
                  <TableCell>{o.retry_count}</TableCell>
                  <TableCell className="text-xs text-muted-foreground">{formatDate(o.created_at)}</TableCell>
                  <TableCell>
                    <Button
                      size="sm"
                      variant="outline"
                      disabled={["failed", "issued", "revoked"].includes(o.status)}
                      onClick={() => setForceFailDialog({ orderId: o.id, open: true })}
                    >
                      {t("cert.orders.forceFail")}
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      <AlertDialog
        open={forceFailDialog.open}
        onOpenChange={(open) => setForceFailDialog((d) => ({ ...d, open }))}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("cert.orders.confirmForceFailTitle")}</AlertDialogTitle>
            <AlertDialogDescription>
              {t("cert.orders.confirmForceFailDesc", { id: forceFailDialog.orderId })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <div className="flex flex-col gap-2">
            <Label htmlFor="force-fail-reason">{t("cert.orders.reasonLabel")}</Label>
            <Input
              id="force-fail-reason"
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              placeholder={t("cert.orders.reasonPlaceholder")}
            />
          </div>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={processing}>{t("cert.orders.cancel")}</AlertDialogCancel>
            <AlertDialogAction onClick={handleForceFail} disabled={processing}>
              {processing ? t("cert.orders.processing") : t("cert.orders.confirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
