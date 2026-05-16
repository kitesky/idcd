"use client"

import { useEffect, useState } from "react"
import Link from "next/link"
import { toast } from "sonner"
import { Badge } from "@/components/ui/badge"
import { Skeleton } from "@/components/ui/skeleton"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { listCerts } from "../cert-api"
import {
  CERT_STATUS_LABELS,
  isExpiringSoon,
  type Cert,
  type CertStatus,
} from "../types"

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

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  })
}

export function CertsClient() {
  const [certs, setCerts] = useState<Cert[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let mounted = true
    listCerts()
      .then((c) => {
        if (mounted) setCerts(c)
      })
      .catch((err) => {
        if (mounted) {
          const msg = err instanceof Error ? err.message : "加载证书失败"
          toast.error(msg)
        }
      })
      .finally(() => {
        if (mounted) setLoading(false)
      })
    return () => {
      mounted = false
    }
  }, [])

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>SAN</TableHead>
          <TableHead>Issuer</TableHead>
          <TableHead>生效</TableHead>
          <TableHead>到期</TableHead>
          <TableHead>状态</TableHead>
          <TableHead className="w-24">操作</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {loading ? (
          Array.from({ length: 4 }).map((_, i) => (
            <TableRow key={i}>
              <TableCell>
                <Skeleton className="h-4 w-48" />
              </TableCell>
              <TableCell>
                <Skeleton className="h-4 w-32" />
              </TableCell>
              <TableCell>
                <Skeleton className="h-4 w-20" />
              </TableCell>
              <TableCell>
                <Skeleton className="h-4 w-20" />
              </TableCell>
              <TableCell>
                <Skeleton className="h-5 w-12 rounded-full" />
              </TableCell>
              <TableCell>
                <Skeleton className="h-4 w-12" />
              </TableCell>
            </TableRow>
          ))
        ) : certs.length === 0 ? (
          <TableRow>
            <TableCell
              colSpan={6}
              className="h-32 text-center text-sm text-muted-foreground"
            >
              暂无已签发证书
            </TableCell>
          </TableRow>
        ) : (
          certs.map((c) => {
            const expSoon = c.status === "active" && isExpiringSoon(c.notAfter)
            return (
              <TableRow key={c.id}>
                <TableCell className="max-w-[260px] truncate">
                  <Link
                    href={`/app/cert/certs/${c.id}`}
                    className="font-mono text-xs hover:underline underline-offset-4"
                  >
                    {c.san.join(", ")}
                  </Link>
                </TableCell>
                <TableCell className="text-sm">{c.issuer}</TableCell>
                <TableCell className="text-sm tabular-nums">
                  {formatDate(c.notBefore)}
                </TableCell>
                <TableCell className="text-sm tabular-nums">
                  <span className="inline-flex items-center gap-2">
                    {formatDate(c.notAfter)}
                    {expSoon && (
                      <Badge variant="warning" className="text-[10px]">
                        即将到期
                      </Badge>
                    )}
                  </span>
                </TableCell>
                <TableCell>{statusBadge(c.status)}</TableCell>
                <TableCell>
                  <Link
                    href={`/app/cert/certs/${c.id}`}
                    className="text-sm text-primary underline-offset-4 hover:underline"
                  >
                    详情
                  </Link>
                </TableCell>
              </TableRow>
            )
          })
        )}
      </TableBody>
    </Table>
  )
}
