"use client"

import { useEffect, useState } from "react"
import Link from "next/link"
import { useRouter } from "next/navigation"
import { toast } from "sonner"
import { AlertCircle, ArrowLeft, RefreshCw, ShieldCheck } from "lucide-react"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
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
import { Separator } from "@/components/ui/separator"
import { Skeleton } from "@/components/ui/skeleton"
import { CertAPIError, getOrder, markManualReady, retryOrder } from "../../cert-api"
import {
  CA_OPTIONS,
  ORDER_STATUS_LABELS,
  TIER_LABELS,
  type CaProvider,
  type Order,
  type OrderStatus,
} from "../../types"

function statusBadge(status: OrderStatus) {
  switch (status) {
    case "issued":
      return <Badge variant="success">{ORDER_STATUS_LABELS[status]}</Badge>
    case "failed":
      return <Badge variant="destructive">{ORDER_STATUS_LABELS[status]}</Badge>
    case "validating":
      return <Badge variant="info">{ORDER_STATUS_LABELS[status]}</Badge>
    case "issuing":
      return <Badge variant="warning">{ORDER_STATUS_LABELS[status]}</Badge>
    case "revoked":
      return (
        <Badge variant="secondary" className="line-through">
          {ORDER_STATUS_LABELS[status]}
        </Badge>
      )
    case "draft":
    default:
      return <Badge variant="outline">{ORDER_STATUS_LABELS[status]}</Badge>
  }
}

function caLabel(ca: CaProvider): string {
  return CA_OPTIONS.find((c) => c.id === ca)?.label ?? ca
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleString("zh-CN")
}

export function OrderDetailClient({ orderId }: { orderId: string }) {
  const router = useRouter()
  const [order, setOrder] = useState<Order | null>(null)
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    let mounted = true
    getOrder(orderId)
      .then((o) => {
        if (mounted) setOrder(o)
      })
      .catch((err) => {
        if (mounted) {
          const msg = err instanceof Error ? err.message : "加载失败"
          toast.error(msg)
        }
      })
      .finally(() => {
        if (mounted) setLoading(false)
      })
    return () => {
      mounted = false
    }
  }, [orderId])

  async function handleRetry() {
    setBusy(true)
    try {
      const updated = await retryOrder(orderId)
      if (updated) {
        setOrder(updated)
        toast.success("已重新提交订单")
      }
    } catch (err) {
      const msg = err instanceof CertAPIError ? err.message : err instanceof Error ? err.message : "重试失败"
      toast.error(msg)
    } finally {
      setBusy(false)
    }
  }

  async function handleConfirmManual() {
    if (!order) return
    const first = order.manualChallenges?.[0]
    const fqdn = first?.recordName ?? ""
    const value = first?.recordValue ?? ""
    if (!fqdn || !value) {
      toast.error("缺少 TXT 记录信息，请联系管理员")
      return
    }
    setBusy(true)
    try {
      const updated = await markManualReady(orderId, fqdn, value)
      if (updated) {
        setOrder(updated)
        toast.success("已通知 worker，正在验证")
      }
    } catch (err) {
      const msg = err instanceof CertAPIError ? err.message : err instanceof Error ? err.message : "确认失败"
      toast.error(msg)
    } finally {
      setBusy(false)
    }
  }

  if (loading) {
    return (
      <div className="space-y-4">
        <Skeleton className="h-6 w-64" />
        <Skeleton className="h-32 w-full rounded-lg" />
        <Skeleton className="h-48 w-full rounded-lg" />
      </div>
    )
  }

  if (!order) {
    return (
      <Alert variant="destructive">
        <AlertCircle className="h-4 w-4" />
        <AlertTitle>订单不存在</AlertTitle>
        <AlertDescription>
          ID {orderId} 找不到对应订单。{" "}
          <Link
            href="/app/cert/orders"
            className="text-primary underline-offset-4 hover:underline"
          >
            返回订单列表
          </Link>
        </AlertDescription>
      </Alert>
    )
  }

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
              <Link href="/app/cert/orders">订单</Link>
            </BreadcrumbLink>
          </BreadcrumbItem>
          <BreadcrumbSeparator />
          <BreadcrumbItem>
            <BreadcrumbPage className="font-mono text-xs">{order.id}</BreadcrumbPage>
          </BreadcrumbItem>
        </BreadcrumbList>
      </Breadcrumb>

      <div className="flex items-start justify-between gap-3">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">
            订单 {order.id}
          </h1>
          <div className="mt-2 flex items-center gap-2">
            {statusBadge(order.status)}
            <Badge variant="outline" className="text-[10px]">
              {TIER_LABELS[order.tier]}
            </Badge>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" onClick={() => router.back()}>
            <ArrowLeft className="mr-2 h-4 w-4" />
            返回
          </Button>
          {order.status === "failed" && (
            <Button size="sm" onClick={handleRetry} disabled={busy}>
              <RefreshCw className="mr-2 h-4 w-4" />
              {busy ? "重试中…" : "重试"}
            </Button>
          )}
          {order.status === "issued" && order.certId && (
            <Button asChild size="sm">
              <Link href={`/app/cert/certs/${order.certId}`}>
                <ShieldCheck className="mr-2 h-4 w-4" />
                查看证书
              </Link>
            </Button>
          )}
        </div>
      </div>

      {order.status === "failed" && order.errorMessage && (
        <Alert variant="destructive">
          <AlertCircle className="h-4 w-4" />
          <AlertTitle>签发失败</AlertTitle>
          <AlertDescription>{order.errorMessage}</AlertDescription>
        </Alert>
      )}

      {order.challenge === "dns01-manual" &&
        order.status === "validating" &&
        order.manualChallenges && (
          <Alert>
            <AlertCircle className="h-4 w-4" />
            <AlertTitle>请添加以下 TXT 记录</AlertTitle>
            <AlertDescription className="mt-2 space-y-3">
              <p className="text-xs text-muted-foreground">
                在你的权威 DNS 后台为每个域名添加 TXT 记录，传播完成（约 1-5
                分钟）后点击「我已添加」。
              </p>
              <div className="space-y-2">
                {order.manualChallenges.map((c) => (
                  <div
                    key={c.recordName}
                    className="rounded-md border bg-muted/30 p-3 font-mono text-xs"
                  >
                    <div className="text-muted-foreground">名称</div>
                    <code className="block break-all">{c.recordName}</code>
                    <div className="mt-2 text-muted-foreground">值</div>
                    <code className="block break-all">{c.recordValue}</code>
                  </div>
                ))}
              </div>
              <Button
                size="sm"
                onClick={handleConfirmManual}
                disabled={busy}
                data-testid="confirm-manual"
              >
                {busy ? "校验中…" : "我已添加，开始验证"}
              </Button>
            </AlertDescription>
          </Alert>
        )}

      <Card>
        <CardHeader>
          <CardTitle className="text-base">订单信息</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3 text-sm">
          <Row label="SAN">
            <div className="flex flex-wrap justify-end gap-1">
              {order.san.map((s) => (
                <Badge key={s} variant="outline" className="font-mono text-xs">
                  {s}
                </Badge>
              ))}
            </div>
          </Row>
          <Separator />
          <Row label="CA">{caLabel(order.ca)}</Row>
          <Separator />
          <Row label="验证方式">
            {order.challenge === "dns01-auto" ? "DNS-01 自动" : "DNS-01 手动"}
          </Row>
          <Separator />
          <Row label="状态">{statusBadge(order.status)}</Row>
          <Separator />
          <Row label="创建时间">{formatDate(order.createdAt)}</Row>
          <Separator />
          <Row label="幂等键">
            <code className="font-mono text-xs">{order.idempotencyKey}</code>
          </Row>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">事件流</CardTitle>
        </CardHeader>
        <CardContent>
          {order.events.length === 0 ? (
            <p className="text-sm text-muted-foreground">暂无事件</p>
          ) : (
            <ol className="space-y-3">
              {order.events.map((e, i) => (
                <li
                  key={`${e.at}-${i}`}
                  className="flex items-start gap-3 text-sm"
                >
                  <span className="shrink-0 font-mono text-xs text-muted-foreground">
                    {formatDate(e.at)}
                  </span>
                  <div className="flex-1">
                    <code className="font-mono text-xs">{e.action}</code>
                    {e.payload && (
                      <pre className="mt-1 overflow-x-auto rounded bg-muted/30 p-2 text-[11px] text-muted-foreground">
                        {JSON.stringify(e.payload)}
                      </pre>
                    )}
                  </div>
                </li>
              ))}
            </ol>
          )}
        </CardContent>
      </Card>
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
      <span className="text-muted-foreground">{label}</span>
      <div className="text-right">{children}</div>
    </div>
  )
}
