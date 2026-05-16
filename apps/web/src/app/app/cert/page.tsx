"use client"

import { useEffect, useState } from "react"
import Link from "next/link"
import { CalendarClock, CheckCircle2, Plus, ShieldCheck } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Progress } from "@/components/ui/progress"
import { Skeleton } from "@/components/ui/skeleton"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { getQuota, listOrders } from "./cert-api"
import {
  CA_OPTIONS,
  ORDER_STATUS_LABELS,
  TIER_LABELS,
  type CaProvider,
  type CertQuota,
  type Order,
  type OrderStatus,
} from "./types"

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
  return new Date(iso).toLocaleString("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  })
}

export default function CertOverviewPage() {
  const [quota, setQuota] = useState<CertQuota | null>(null)
  const [orders, setOrders] = useState<Order[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let mounted = true
    Promise.all([getQuota(), listOrders()])
      .then(([q, o]) => {
        if (!mounted) return
        setQuota(q)
        setOrders(o.slice(0, 5))
      })
      .finally(() => {
        if (mounted) setLoading(false)
      })
    return () => {
      mounted = false
    }
  }, [])

  const usedPercent = quota ? Math.round((quota.used / quota.limit) * 100) : 0

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">证书总览</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            申请、续签和管理来自 Let&apos;s Encrypt / ZeroSSL / Buypass 等 CA 的免费 TLS 证书。
          </p>
        </div>
        <Button asChild className="h-8">
          <Link href="/app/cert/new">
            <Plus className="mr-2 h-4 w-4" />
            申请新证书
          </Link>
        </Button>
      </div>

      {/* 配额卡片 */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="flex items-center gap-2 text-sm font-medium text-muted-foreground">
              <ShieldCheck className="h-4 w-4" />
              本月已用配额
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-2">
            {loading || !quota ? (
              <Skeleton className="h-9 w-24" />
            ) : (
              <>
                <p className="text-3xl font-bold tabular-nums">
                  {quota.used}
                  <span className="text-base font-normal text-muted-foreground">
                    {" "}
                    / {quota.limit}
                  </span>
                </p>
                <Progress value={usedPercent} />
              </>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="flex items-center gap-2 text-sm font-medium text-muted-foreground">
              <CalendarClock className="h-4 w-4 text-warning" />
              30 天内到期
            </CardTitle>
          </CardHeader>
          <CardContent>
            {loading || !quota ? (
              <Skeleton className="h-9 w-12" />
            ) : (
              <p className="text-3xl font-bold tabular-nums text-warning">
                {quota.expiringSoon}
              </p>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="flex items-center gap-2 text-sm font-medium text-muted-foreground">
              <CheckCircle2 className="h-4 w-4 text-success" />
              本月签发成功率
            </CardTitle>
          </CardHeader>
          <CardContent>
            {loading || !quota ? (
              <Skeleton className="h-9 w-16" />
            ) : (
              <p className="text-3xl font-bold tabular-nums text-success">
                {quota.monthlySuccessRate}%
              </p>
            )}
          </CardContent>
        </Card>
      </div>

      {/* 最近订单 */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">最近订单</CardTitle>
          <CardDescription>
            <Link
              href="/app/cert/orders"
              className="text-primary underline-offset-4 hover:underline"
            >
              查看全部订单
            </Link>
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>SAN</TableHead>
                <TableHead>CA</TableHead>
                <TableHead>状态</TableHead>
                <TableHead className="hidden md:table-cell">创建时间</TableHead>
                <TableHead className="w-20">操作</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {loading ? (
                Array.from({ length: 3 }).map((_, i) => (
                  <TableRow key={i}>
                    <TableCell>
                      <Skeleton className="h-4 w-40" />
                    </TableCell>
                    <TableCell>
                      <Skeleton className="h-4 w-24" />
                    </TableCell>
                    <TableCell>
                      <Skeleton className="h-5 w-16 rounded-full" />
                    </TableCell>
                    <TableCell className="hidden md:table-cell">
                      <Skeleton className="h-4 w-20" />
                    </TableCell>
                    <TableCell>
                      <Skeleton className="h-4 w-12" />
                    </TableCell>
                  </TableRow>
                ))
              ) : orders.length === 0 ? (
                <TableRow>
                  <TableCell
                    colSpan={5}
                    className="h-24 text-center text-sm text-muted-foreground"
                  >
                    暂无订单，
                    <Link
                      href="/app/cert/new"
                      className="text-primary underline-offset-4 hover:underline"
                    >
                      去申请一张证书
                    </Link>
                  </TableCell>
                </TableRow>
              ) : (
                orders.map((o) => (
                  <TableRow key={o.id}>
                    <TableCell className="max-w-[260px] truncate font-mono text-xs">
                      {o.san.join(", ")}
                      {o.tier !== "free" && (
                        <Badge variant="outline" className="ml-2 text-[10px]">
                          {TIER_LABELS[o.tier]}
                        </Badge>
                      )}
                    </TableCell>
                    <TableCell className="text-sm">{caLabel(o.ca)}</TableCell>
                    <TableCell>{statusBadge(o.status)}</TableCell>
                    <TableCell className="hidden md:table-cell text-sm text-muted-foreground">
                      {formatDate(o.createdAt)}
                    </TableCell>
                    <TableCell>
                      <Link
                        href={`/app/cert/orders/${o.id}`}
                        className="text-sm text-primary underline-offset-4 hover:underline"
                      >
                        详情
                      </Link>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  )
}
