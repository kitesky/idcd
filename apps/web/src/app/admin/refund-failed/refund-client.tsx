"use client"

import { useState, useCallback } from "react"
import { useTranslations } from "next-intl"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { AlertTriangle, RefreshCw } from "lucide-react"

export interface RefundFailedPayment {
  id: string
  user_id: string
  invoice_id?: string | null
  amount_cents: number
  currency: string
  refund_retry_count: number
  refund_failed_at?: string | null
  created_at: string
}

function formatAmount(cents: number, currency: string) {
  return `${currency} ${(cents / 100).toFixed(2)}`
}

function formatDate(iso?: string | null) {
  if (!iso) return "—"
  try { return new Date(iso).toLocaleString(undefined, { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" }) }
  catch { return iso }
}

export function RefundClient({ initialPayments }: { initialPayments: RefundFailedPayment[] }) {
  const t = useTranslations("admin")
  const [payments, setPayments] = useState(initialPayments)
  const [retrying, setRetrying] = useState<Record<string, boolean>>({})
  const [errors,   setErrors]   = useState<Record<string, string>>({})

  const handleRetry = useCallback(async (id: string) => {
    setRetrying(p => ({ ...p, [id]: true }))
    setErrors(p => { const n = { ...p }; delete n[id]; return n })
    try {
      const res = await fetch(`/api/admin/refund-failed/${id}/retry`, { method: "POST" })
      if (!res.ok) {
        const body = await res.json().catch(() => ({}))
        throw new Error(body?.error?.message ?? `HTTP ${res.status}`)
      }
      setPayments(p => p.filter(x => x.id !== id))
    } catch (err: unknown) {
      setErrors(p => ({ ...p, [id]: err instanceof Error ? err.message : String(err) }))
    } finally {
      setRetrying(p => ({ ...p, [id]: false }))
    }
  }, [])

  return (
    <div className="space-y-4">
      {payments.length > 0 ? (
        <Alert variant="destructive">
          <AlertTriangle className="h-4 w-4" />
          <AlertTitle>{t("refundFailed.alertTitle")}</AlertTitle>
          <AlertDescription>
            {t("refundFailed.alertDesc", { count: payments.length })}
          </AlertDescription>
        </Alert>
      ) : (
        <Alert>
          <AlertTitle>{t("refundFailed.emptyTitle")}</AlertTitle>
          <AlertDescription>{t("refundFailed.emptyDesc")}</AlertDescription>
        </Alert>
      )}

      <Card>
        <CardHeader><CardTitle>{t("refundFailed.listTitle")}</CardTitle></CardHeader>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("refundFailed.table.paymentId")}</TableHead>
                <TableHead>{t("refundFailed.table.userId")}</TableHead>
                <TableHead>{t("refundFailed.table.amount")}</TableHead>
                <TableHead>{t("refundFailed.table.failedAt")}</TableHead>
                <TableHead>{t("refundFailed.table.retryCount")}</TableHead>
                <TableHead>{t("refundFailed.table.actions")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {payments.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={6} className="py-8 text-center text-muted-foreground">
                    {t("refundFailed.noData")}
                  </TableCell>
                </TableRow>
              ) : payments.map(p => (
                <TableRow key={p.id}>
                  <TableCell className="font-mono text-xs">{p.id}</TableCell>
                  <TableCell className="font-mono text-xs">{p.user_id}</TableCell>
                  <TableCell>
                    <Badge variant="destructive" className="font-mono">
                      {formatAmount(p.amount_cents, p.currency)}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-xs text-muted-foreground">{formatDate(p.refund_failed_at)}</TableCell>
                  <TableCell>{p.refund_retry_count} {t("refundFailed.retryCountUnit")}</TableCell>
                  <TableCell>
                    <div className="flex flex-col gap-1">
                      <Button size="sm" variant="outline" disabled={retrying[p.id]} onClick={() => handleRetry(p.id)}>
                        {retrying[p.id]
                          ? <><RefreshCw className="mr-1 h-3 w-3 animate-spin" />{t("refundFailed.retrying")}</>
                          : t("refundFailed.manualRetry")}
                      </Button>
                      {errors[p.id] && <span className="text-xs text-destructive">{errors[p.id]}</span>}
                    </div>
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
