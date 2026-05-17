"use client"

import { useCallback, useState } from "react"
import { useTranslations } from "next-intl"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { getDNSHealth, formatRate, type AdminDNSHealthResponse } from "../admin-cert-api"

function rateVariant(rate: number): "default" | "secondary" | "destructive" | "outline" {
  if (rate < 0) return "outline"
  if (rate >= 0.95) return "secondary"
  if (rate >= 0.9) return "default"
  return "destructive"
}

export function DNSHealthClient({ initial }: { initial: AdminDNSHealthResponse }) {
  const t = useTranslations("admin")
  const [data, setData] = useState<AdminDNSHealthResponse>(initial)
  const [loading, setLoading] = useState(false)

  const refresh = useCallback(async () => {
    setLoading(true)
    try {
      const fresh = await getDNSHealth()
      if (fresh) setData(fresh)
    } finally {
      setLoading(false)
    }
  }, [])

  return (
    <div className="space-y-4">
      <div className="flex justify-end">
        <Button variant="outline" disabled={loading} onClick={() => void refresh()}>
          {loading ? t("cert.orders.loading") : t("cert.dnsHealth.refresh")}
        </Button>
      </div>
      <Card>
        <CardHeader>
          <CardTitle>{t("cert.dnsHealth.title")}</CardTitle>
        </CardHeader>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("cert.dnsHealth.provider")}</TableHead>
                <TableHead>{t("cert.dnsHealth.successRate")}</TableHead>
                <TableHead>{t("cert.dnsHealth.samples")}</TableHead>
                <TableHead>{t("cert.dnsHealth.window")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {data.rows.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={4} className="py-8 text-center text-muted-foreground">
                    {t("cert.dnsHealth.noData")}
                  </TableCell>
                </TableRow>
              ) : data.rows.map((r) => (
                <TableRow key={r.provider}>
                  <TableCell className="font-mono">{r.provider}</TableCell>
                  <TableCell>
                    <Badge variant={rateVariant(r.success_rate)} className="font-mono">
                      {formatRate(r.success_rate, t("cert.dnsHealth.unknown"))}
                    </Badge>
                  </TableCell>
                  <TableCell className="font-mono">{r.samples}</TableCell>
                  <TableCell className="font-mono">{r.window_hours}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  )
}
