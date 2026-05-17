"use client"

import { useCallback, useEffect, useRef, useState } from "react"
import Link from "next/link"
import {
  AlertCircle,
  Archive,
  Download,
  RefreshCcw,
  ShieldCheck,
} from "lucide-react"

import {
  Alert,
  AlertDescription,
  AlertTitle,
  Badge,
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
  Button,
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  Separator,
  Skeleton,
} from "@/components/ui"
import {
  getVerdictOrder,
  getVerdictReport,
  isPollingStatus,
  statusBadgeVariant,
  VERDICT_STATUS_LABELS,
  VERDICT_TEMPLATE_LABELS,
  type VerdictOrder,
  type VerdictReport,
} from "@/lib/api/verdict"

const POLL_INTERVAL_MS = 5_000

interface Props {
  orderId: string
}

export function VerdictOrderDetailClient({ orderId }: Props) {
  const [order, setOrder] = useState<VerdictOrder | null>(null)
  const [report, setReport] = useState<VerdictReport | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null)

  const fetchOrder = useCallback(async () => {
    try {
      const o = await getVerdictOrder(orderId)
      setOrder(o)
      setError(null)

      // Once an order has a report_id (i.e. delivered), fetch the report
      // metadata once. We do not poll for report metadata — it is immutable
      // once issued.
      if (o.report_id && !report) {
        try {
          const r = await getVerdictReport(o.report_id)
          setReport(r)
        } catch (err) {
          // Surface report-fetch errors but keep the order visible.
          setError(err instanceof Error ? err.message : "加载报告详情失败")
        }
      }
      return o
    } catch (err) {
      setError(err instanceof Error ? err.message : "加载订单失败")
      return null
    } finally {
      setLoading(false)
    }
  }, [orderId, report])

  // Initial load + polling effect. Polling runs every 5s while the order is
  // in a transient state (paid / generating). We tear down the interval as
  // soon as the order resolves to a terminal state to avoid wasted network.
  useEffect(() => {
    let cancelled = false

    void (async () => {
      const o = await fetchOrder()
      if (cancelled || !o) return

      if (isPollingStatus(o.status)) {
        intervalRef.current = setInterval(async () => {
          const next = await fetchOrder()
          if (next && !isPollingStatus(next.status) && intervalRef.current) {
            clearInterval(intervalRef.current)
            intervalRef.current = null
          }
        }, POLL_INTERVAL_MS)
      }
    })()

    return () => {
      cancelled = true
      if (intervalRef.current) {
        clearInterval(intervalRef.current)
        intervalRef.current = null
      }
    }
    // We intentionally exclude `fetchOrder` from deps because it would
    // re-run the effect whenever `report` changes, restarting polling.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [orderId])

  if (loading) {
    return (
      <div className="space-y-4" data-testid="loading">
        <Skeleton className="h-8 w-64" />
        <Skeleton className="h-48 w-full rounded-lg" />
      </div>
    )
  }

  if (error && !order) {
    return (
      <Alert variant="destructive" data-testid="order-error">
        <AlertCircle className="h-4 w-4" />
        <AlertTitle>无法加载订单</AlertTitle>
        <AlertDescription>{error}</AlertDescription>
      </Alert>
    )
  }

  if (!order) {
    return null
  }

  return (
    <div className="max-w-3xl space-y-6" data-testid="verdict-order-detail">
      <Breadcrumb>
        <BreadcrumbList>
          <BreadcrumbItem>
            <BreadcrumbLink asChild>
              <Link href="/app/verdict/new">证据报告</Link>
            </BreadcrumbLink>
          </BreadcrumbItem>
          <BreadcrumbSeparator />
          <BreadcrumbItem>
            <BreadcrumbPage>订单详情</BreadcrumbPage>
          </BreadcrumbItem>
        </BreadcrumbList>
      </Breadcrumb>
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">证据报告订单</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            订单号：<code className="font-mono text-xs">{order.id}</code>
          </p>
        </div>
        <Badge
          variant={statusBadgeVariant(order.status)}
          data-testid="status-badge"
          className="text-sm"
        >
          {VERDICT_STATUS_LABELS[order.status]}
        </Badge>
      </div>

      {isPollingStatus(order.status) && (
        <Alert data-testid="polling-alert">
          <RefreshCcw className="h-4 w-4 animate-spin" />
          <AlertTitle>报告生成中</AlertTitle>
          <AlertDescription>
            页面每 5 秒自动刷新一次状态，无需手动等待；完成后可直接下载 PDF。
          </AlertDescription>
        </Alert>
      )}

      {order.status === "failed" && (
        <Alert variant="destructive" data-testid="failed-alert">
          <AlertCircle className="h-4 w-4" />
          <AlertTitle>报告生成失败</AlertTitle>
          <AlertDescription>
            系统会自动重试；如长时间未恢复，请联系客服，款项将按 SLA 处理（参见 D12 三档 SLA）。
          </AlertDescription>
        </Alert>
      )}

      {order.status === "refunded" && (
        <Alert data-testid="refunded-alert">
          <AlertTitle>订单已退款</AlertTitle>
          <AlertDescription>退款已发起，到账时间视支付渠道而定。</AlertDescription>
        </Alert>
      )}

      <Card>
        <CardHeader>
          <CardTitle className="text-base">订单信息</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3 text-sm">
          <MetaRow label="模板">
            <Badge variant="outline">{VERDICT_TEMPLATE_LABELS[order.template]}</Badge>
          </MetaRow>
          <MetaRow label="目标">
            <span className="font-mono text-xs">{order.target}</span>
          </MetaRow>
          <MetaRow label="时间窗">
            <span className="text-xs">
              {formatLocal(order.time_window_start)} → {formatLocal(order.time_window_end)}
            </span>
          </MetaRow>
          <MetaRow label="价格">
            <span>¥{order.price_cny.toFixed(2)}</span>
          </MetaRow>
          {order.paid_at && (
            <MetaRow label="支付时间">
              <span className="text-xs">{formatLocal(order.paid_at)}</span>
            </MetaRow>
          )}
          {order.delivered_at && (
            <MetaRow label="交付时间">
              <span className="text-xs">{formatLocal(order.delivered_at)}</span>
            </MetaRow>
          )}
        </CardContent>
      </Card>

      {order.status === "delivered" && report && (
        <Card data-testid="report-card">
          <CardHeader>
            <CardTitle className="text-base">报告交付</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4 text-sm">
            <MetaRow label="内容哈希">
              <span className="font-mono text-[11px] break-all">{report.content_hash}</span>
            </MetaRow>
            <MetaRow label="TSA 时间戳">
              <span className="text-xs">
                {report.tsa_provider} · {formatLocal(report.tsa_time)}
              </span>
            </MetaRow>
            <MetaRow label="自检状态">
              <Badge variant={report.self_verify_status === "pass" ? "success" : "warning"}>
                {report.self_verify_status}
              </Badge>
            </MetaRow>

            <Separator />

            <div className="flex flex-wrap gap-2">
              <Button asChild data-testid="download-pdf-btn">
                <a href={report.pdf_url} target="_blank" rel="noreferrer noopener">
                  <Download className="mr-2 h-4 w-4" /> 下载 PDF
                </a>
              </Button>
              {report.archived_url && (
                <Button asChild variant="outline" data-testid="archive-btn">
                  <a href={report.archived_url} target="_blank" rel="noreferrer noopener">
                    <Archive className="mr-2 h-4 w-4" /> 归档副本
                  </a>
                </Button>
              )}
              <Button asChild variant="secondary" data-testid="verify-cta">
                <Link href="/verify">
                  <ShieldCheck className="mr-2 h-4 w-4" /> 验证此报告
                </Link>
              </Button>
            </div>
          </CardContent>
        </Card>
      )}

      {error && order && (
        <Alert variant="destructive">
          <AlertCircle className="h-4 w-4" />
          <AlertTitle>刷新出错</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}
    </div>
  )
}

function MetaRow({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-4">
      <span className="text-muted-foreground">{label}</span>
      <div className="text-right">{children}</div>
    </div>
  )
}

function formatLocal(iso: string): string {
  try {
    return new Date(iso).toLocaleString("zh-CN", { hour12: false })
  } catch {
    return iso
  }
}
