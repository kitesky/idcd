"use client"

import { useEffect, useMemo, useState } from "react"
import Link from "next/link"
import { Plus } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Skeleton } from "@/components/ui/skeleton"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { listOrders } from "../cert-api"
import {
  CA_OPTIONS,
  ORDER_STATUS_LABELS,
  TIER_LABELS,
  type CaProvider,
  type Order,
  type OrderStatus,
} from "../types"

const STATUS_FILTERS: { value: OrderStatus | "all"; label: string }[] = [
  { value: "all", label: "全部状态" },
  { value: "draft", label: ORDER_STATUS_LABELS.draft },
  { value: "validating", label: ORDER_STATUS_LABELS.validating },
  { value: "issuing", label: ORDER_STATUS_LABELS.issuing },
  { value: "issued", label: ORDER_STATUS_LABELS.issued },
  { value: "failed", label: ORDER_STATUS_LABELS.failed },
  { value: "revoked", label: ORDER_STATUS_LABELS.revoked },
]

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
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  })
}

export function OrdersClient() {
  const [orders, setOrders] = useState<Order[]>([])
  const [loading, setLoading] = useState(true)
  const [statusFilter, setStatusFilter] = useState<OrderStatus | "all">("all")
  const [search, setSearch] = useState("")

  useEffect(() => {
    let mounted = true
    listOrders()
      .then((o) => {
        if (mounted) setOrders(o)
      })
      .finally(() => {
        if (mounted) setLoading(false)
      })
    return () => {
      mounted = false
    }
  }, [])

  const filtered = useMemo(() => {
    return orders.filter((o) => {
      if (statusFilter !== "all" && o.status !== statusFilter) return false
      if (
        search &&
        !o.san.join(",").toLowerCase().includes(search.toLowerCase())
      )
        return false
      return true
    })
  }, [orders, statusFilter, search])

  return (
    <div className="space-y-4">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
          <Input
            placeholder="搜索 SAN…"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="h-8 w-full sm:w-64"
          />
          <Select
            value={statusFilter}
            onValueChange={(v) => setStatusFilter(v as OrderStatus | "all")}
          >
            <SelectTrigger className="h-8 w-full sm:w-36">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {STATUS_FILTERS.map((f) => (
                <SelectItem key={f.value} value={f.value}>
                  {f.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <Button asChild className="h-8">
          <Link href="/app/cert/new">
            <Plus className="mr-2 h-4 w-4" />
            申请新证书
          </Link>
        </Button>
      </div>

      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>SAN</TableHead>
            <TableHead>档位</TableHead>
            <TableHead>CA</TableHead>
            <TableHead>状态</TableHead>
            <TableHead className="hidden md:table-cell">创建时间</TableHead>
            <TableHead className="w-20">操作</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {loading ? (
            Array.from({ length: 5 }).map((_, i) => (
              <TableRow key={i}>
                <TableCell>
                  <Skeleton className="h-4 w-48" />
                </TableCell>
                <TableCell>
                  <Skeleton className="h-5 w-12 rounded-full" />
                </TableCell>
                <TableCell>
                  <Skeleton className="h-4 w-24" />
                </TableCell>
                <TableCell>
                  <Skeleton className="h-5 w-16 rounded-full" />
                </TableCell>
                <TableCell className="hidden md:table-cell">
                  <Skeleton className="h-4 w-24" />
                </TableCell>
                <TableCell>
                  <Skeleton className="h-4 w-12" />
                </TableCell>
              </TableRow>
            ))
          ) : filtered.length === 0 ? (
            <TableRow>
              <TableCell
                colSpan={6}
                className="h-32 text-center text-sm text-muted-foreground"
              >
                {search || statusFilter !== "all"
                  ? "没有匹配的订单"
                  : "暂无订单"}
              </TableCell>
            </TableRow>
          ) : (
            filtered.map((o) => (
              <TableRow key={o.id}>
                <TableCell className="max-w-[260px] truncate">
                  <Link
                    href={`/app/cert/orders/${o.id}`}
                    className="font-mono text-xs hover:underline underline-offset-4"
                  >
                    {o.san.join(", ")}
                  </Link>
                </TableCell>
                <TableCell>
                  <Badge variant="outline" className="text-[10px]">
                    {TIER_LABELS[o.tier]}
                  </Badge>
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
    </div>
  )
}
