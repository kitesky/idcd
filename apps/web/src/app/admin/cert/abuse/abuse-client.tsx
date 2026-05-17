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
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { banAccount } from "../admin-cert-api"

export function AbuseClient() {
  const t = useTranslations("admin")
  const [accountId, setAccountId] = useState("")
  const [reason, setReason] = useState("")
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [processing, setProcessing] = useState(false)
  const [feedback, setFeedback] = useState<{ ok: boolean; message: string } | null>(null)

  const parsedId = (() => {
    const n = Number.parseInt(accountId, 10)
    return Number.isFinite(n) && n > 0 ? n : null
  })()

  const handleConfirm = useCallback(async () => {
    if (parsedId == null) return
    setProcessing(true)
    try {
      const res = await banAccount(parsedId, reason || "admin ban")
      if (res.ok) {
        setFeedback({ ok: true, message: t("cert.abuse.success", { id: parsedId }) })
        setAccountId("")
        setReason("")
      } else {
        setFeedback({ ok: false, message: t("cert.abuse.error", { message: res.message }) })
      }
    } finally {
      setProcessing(false)
      setConfirmOpen(false)
    }
  }, [parsedId, reason, t])

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>{t("cert.abuse.title")}</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid gap-4 md:grid-cols-2">
            <div className="flex flex-col gap-1">
              <Label htmlFor="ban-account-id">{t("cert.abuse.accountIdLabel")}</Label>
              <Input
                id="ban-account-id"
                value={accountId}
                onChange={(e) => setAccountId(e.target.value)}
                placeholder={t("cert.abuse.accountIdPlaceholder")}
                inputMode="numeric"
              />
            </div>
            <div className="flex flex-col gap-1">
              <Label htmlFor="ban-reason">{t("cert.abuse.reasonLabel")}</Label>
              <Input
                id="ban-reason"
                value={reason}
                onChange={(e) => setReason(e.target.value)}
                placeholder={t("cert.abuse.reasonPlaceholder")}
              />
            </div>
          </div>
          <div className="mt-4">
            <Button
              variant="destructive"
              disabled={parsedId == null || processing}
              onClick={() => setConfirmOpen(true)}
            >
              {t("cert.abuse.banButton")}
            </Button>
          </div>
          {feedback && (
            <Alert variant={feedback.ok ? "default" : "destructive"} className="mt-4">
              <AlertTitle>{feedback.ok ? t("cert.abuse.success", { id: parsedId ?? 0 }) : t("cert.abuse.banButton")}</AlertTitle>
              <AlertDescription>{feedback.message}</AlertDescription>
            </Alert>
          )}
        </CardContent>
      </Card>

      <AlertDialog open={confirmOpen} onOpenChange={setConfirmOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("cert.abuse.confirmTitle", { id: parsedId ?? 0 })}</AlertDialogTitle>
            <AlertDialogDescription>{t("cert.abuse.confirmDesc")}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={processing}>{t("cert.abuse.cancel")}</AlertDialogCancel>
            <AlertDialogAction onClick={handleConfirm} disabled={processing}>
              {processing ? t("cert.abuse.processing") : t("cert.abuse.confirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
