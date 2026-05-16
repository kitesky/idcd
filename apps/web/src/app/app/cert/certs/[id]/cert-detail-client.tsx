"use client"

import { useEffect, useState } from "react"
import Link from "next/link"
import { toast } from "sonner"
import { AlertCircle, ArrowLeft, Download, ShieldOff } from "lucide-react"
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/components/ui/alert"
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
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Label } from "@/components/ui/label"
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Separator } from "@/components/ui/separator"
import { Skeleton } from "@/components/ui/skeleton"
import { downloadCert, getCert, revokeCert } from "../../cert-api"
import {
  CERT_STATUS_LABELS,
  REVOKE_REASON_LABELS,
  isExpiringSoon,
  type Cert,
  type CertStatus,
  type DownloadFormat,
  type RevokeReason,
} from "../../types"

function statusBadge(status: CertStatus) {
  switch (status) {
    case "active":
      return <Badge variant="success">{CERT_STATUS_LABELS[status]}</Badge>
    case "expired":
      return <Badge variant="destructive">{CERT_STATUS_LABELS[status]}</Badge>
    case "revoked":
      return (
        <Badge variant="secondary" className="line-through">
          {CERT_STATUS_LABELS[status]}
        </Badge>
      )
  }
}

function formatDateTime(iso: string): string {
  return new Date(iso).toLocaleString("zh-CN")
}

const FORMAT_LABELS: Record<DownloadFormat, string> = {
  pem: "PEM (.pem)",
  pkcs12: "PKCS#12 (.p12)",
  "nginx-fullchain": "nginx fullchain.pem",
}

export function CertDetailClient({ certId }: { certId: string }) {
  const [cert, setCert] = useState<Cert | null>(null)
  const [loading, setLoading] = useState(true)
  const [downloadOpen, setDownloadOpen] = useState(false)
  const [format, setFormat] = useState<DownloadFormat>("pem")
  const [revokeOpen, setRevokeOpen] = useState(false)
  const [revokeReason, setRevokeReason] = useState<RevokeReason>("unspecified")
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    let mounted = true
    getCert(certId)
      .then((c) => {
        if (mounted) setCert(c)
      })
      .finally(() => {
        if (mounted) setLoading(false)
      })
    return () => {
      mounted = false
    }
  }, [certId])

  async function handleDownload() {
    if (!cert) return
    setBusy(true)
    try {
      const { url, filename } = await downloadCert(cert.id, format)
      // Trigger an anchor click to surface the mock data URL as a download.
      const a = document.createElement("a")
      a.href = url
      a.download = filename
      document.body.appendChild(a)
      a.click()
      a.remove()
      toast.success(`已开始下载 ${filename}`)
      setDownloadOpen(false)
    } finally {
      setBusy(false)
    }
  }

  async function handleRevoke() {
    if (!cert) return
    setBusy(true)
    try {
      const updated = await revokeCert(cert.id, revokeReason)
      if (updated) {
        setCert(updated)
        toast.success("证书已撤销")
      }
      setRevokeOpen(false)
    } finally {
      setBusy(false)
    }
  }

  if (loading) {
    return (
      <div className="space-y-4">
        <Skeleton className="h-6 w-64" />
        <Skeleton className="h-64 w-full rounded-lg" />
      </div>
    )
  }

  if (!cert) {
    return (
      <Alert variant="destructive">
        <AlertCircle className="h-4 w-4" />
        <AlertTitle>证书不存在</AlertTitle>
        <AlertDescription>
          ID {certId} 找不到对应证书。{" "}
          <Link
            href="/app/cert/certs"
            className="text-primary underline-offset-4 hover:underline"
          >
            返回证书列表
          </Link>
        </AlertDescription>
      </Alert>
    )
  }

  const expSoon = cert.status === "active" && isExpiringSoon(cert.notAfter)

  return (
    <div className="space-y-6">
      <Breadcrumb>
        <BreadcrumbList>
          <BreadcrumbItem>
            <BreadcrumbLink asChild>
              <Link href="/app/cert">证书</Link>
            </BreadcrumbLink>
          </BreadcrumbItem>
          <BreadcrumbSeparator />
          <BreadcrumbItem>
            <BreadcrumbLink asChild>
              <Link href="/app/cert/certs">已签证书</Link>
            </BreadcrumbLink>
          </BreadcrumbItem>
          <BreadcrumbSeparator />
          <BreadcrumbItem>
            <BreadcrumbPage className="font-mono text-xs">{cert.id}</BreadcrumbPage>
          </BreadcrumbItem>
        </BreadcrumbList>
      </Breadcrumb>

      <div className="flex items-start justify-between gap-3">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">
            {cert.san[0]}
            {cert.san.length > 1 && (
              <span className="ml-2 text-base font-normal text-muted-foreground">
                +{cert.san.length - 1}
              </span>
            )}
          </h1>
          <div className="mt-2 flex flex-wrap items-center gap-2">
            {statusBadge(cert.status)}
            {expSoon && <Badge variant="warning">即将到期</Badge>}
          </div>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" asChild>
            <Link href="/app/cert/certs">
              <ArrowLeft className="mr-2 h-4 w-4" />
              返回
            </Link>
          </Button>
          {cert.status === "active" && (
            <>
              <Button
                size="sm"
                onClick={() => setDownloadOpen(true)}
                data-testid="open-download"
              >
                <Download className="mr-2 h-4 w-4" />
                下载
              </Button>
              <Button
                size="sm"
                variant="destructive"
                onClick={() => setRevokeOpen(true)}
                data-testid="open-revoke"
              >
                <ShieldOff className="mr-2 h-4 w-4" />
                撤销
              </Button>
            </>
          )}
        </div>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">证书信息</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3 text-sm">
          <Row label="SAN">
            <div className="flex flex-wrap justify-end gap-1">
              {cert.san.map((s) => (
                <Badge key={s} variant="outline" className="font-mono text-xs">
                  {s}
                </Badge>
              ))}
            </div>
          </Row>
          <Separator />
          <Row label="Issuer">{cert.issuer}</Row>
          <Separator />
          <Row label="Serial">
            <code className="font-mono text-xs break-all">{cert.serial}</code>
          </Row>
          <Separator />
          <Row label="SHA-256 Fingerprint">
            <code className="font-mono text-xs break-all">
              {cert.fingerprintSha256}
            </code>
          </Row>
          <Separator />
          <Row label="生效">{formatDateTime(cert.notBefore)}</Row>
          <Separator />
          <Row label="到期">{formatDateTime(cert.notAfter)}</Row>
          <Separator />
          <Row label="状态">{statusBadge(cert.status)}</Row>
          <Separator />
          <Row label="订单">
            <Link
              href={`/app/cert/orders/${cert.orderId}`}
              className="font-mono text-xs text-primary underline-offset-4 hover:underline"
            >
              {cert.orderId}
            </Link>
          </Row>
        </CardContent>
      </Card>

      {/* 下载弹窗 */}
      <Dialog open={downloadOpen} onOpenChange={setDownloadOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>下载证书</DialogTitle>
            <DialogDescription>选择需要的导出格式。</DialogDescription>
          </DialogHeader>
          <RadioGroup
            value={format}
            onValueChange={(v) => setFormat(v as DownloadFormat)}
            className="grid gap-3"
          >
            {(Object.keys(FORMAT_LABELS) as DownloadFormat[]).map((f) => (
              <Label
                key={f}
                htmlFor={`fmt-${f}`}
                className="flex cursor-pointer items-center gap-3 rounded-md border p-3 has-[[data-state=checked]]:border-primary"
              >
                <RadioGroupItem id={`fmt-${f}`} value={f} />
                <span>{FORMAT_LABELS[f]}</span>
              </Label>
            ))}
          </RadioGroup>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setDownloadOpen(false)}
              disabled={busy}
            >
              取消
            </Button>
            <Button
              onClick={handleDownload}
              disabled={busy}
              data-testid="confirm-download"
            >
              确认下载
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* 撤销确认 */}
      <AlertDialog open={revokeOpen} onOpenChange={setRevokeOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>撤销证书？</AlertDialogTitle>
            <AlertDialogDescription>
              撤销后无法恢复。CA 会立即将证书加入 CRL / OCSP，吊销生效后浏览器会拒绝该证书。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <div className="space-y-2">
            <Label htmlFor="revoke-reason">撤销原因</Label>
            <Select
              value={revokeReason}
              onValueChange={(v) => setRevokeReason(v as RevokeReason)}
            >
              <SelectTrigger id="revoke-reason">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {(Object.keys(REVOKE_REASON_LABELS) as RevokeReason[]).map((r) => (
                  <SelectItem key={r} value={r}>
                    {REVOKE_REASON_LABELS[r]}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={busy}>取消</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              onClick={handleRevoke}
              disabled={busy}
            >
              确认撤销
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}

function Row({
  label,
  children,
}: {
  label: string
  children: React.ReactNode
}) {
  return (
    <div className="flex items-start justify-between gap-4">
      <span className="shrink-0 text-muted-foreground">{label}</span>
      <div className="text-right">{children}</div>
    </div>
  )
}
