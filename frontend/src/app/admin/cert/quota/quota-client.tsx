"use client"

import { useCallback, useState } from "react"
import { useTranslations } from "next-intl"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Progress } from "@/components/ui/progress"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { getCAQuota, formatPercent, type AdminCAQuotaResponse } from "../admin-cert-api"

export function CAQuotaClient({ initial }: { initial: AdminCAQuotaResponse }) {
  const t = useTranslations("admin")
  const [data, setData] = useState<AdminCAQuotaResponse>(initial)
  const [loading, setLoading] = useState(false)

  const refresh = useCallback(async () => {
    setLoading(true)
    try {
      const fresh = await getCAQuota()
      if (fresh) setData(fresh)
    } finally {
      setLoading(false)
    }
  }, [])

  const thresholdPct = Math.round(data.switch_threshold * 100)

  return (
    <div className="space-y-4">
      <div className="flex justify-end">
        <Button variant="outline" disabled={loading} onClick={() => void refresh()}>
          {loading ? t("cert.orders.loading") : t("cert.quota.refresh")}
        </Button>
      </div>
      <Card>
        <CardHeader>
          <CardTitle>{t("cert.quota.title")}</CardTitle>
        </CardHeader>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("cert.quota.ca")}</TableHead>
                <TableHead>{t("cert.quota.per3h")}</TableHead>
                <TableHead>{t("cert.quota.perWeek")}</TableHead>
                <TableHead>{t("cert.orders.table.status")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {data.rows.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={4} className="py-8 text-center text-muted-foreground">
                    {t("cert.quota.noData")}
                  </TableCell>
                </TableRow>
              ) : data.rows.map((r) => (
                <TableRow key={r.ca}>
                  <TableCell className="font-mono">{r.ca}</TableCell>
                  <TableCell>
                    <div className="flex items-center gap-2">
                      <Progress
                        value={Math.min(100, Math.max(0, r.per_account_3h * 100))}
                        className="w-32"
                      />
                      <span className="font-mono text-xs">{formatPercent(r.per_account_3h)}</span>
                    </div>
                  </TableCell>
                  <TableCell>
                    <div className="flex items-center gap-2">
                      <Progress
                        value={Math.min(100, Math.max(0, r.per_registered_domain * 100))}
                        className="w-32"
                      />
                      <span className="font-mono text-xs">{formatPercent(r.per_registered_domain)}</span>
                    </div>
                  </TableCell>
                  <TableCell>
                    {r.err ? (
                      <Badge variant="outline">{t("cert.quota.unknown")}</Badge>
                    ) : r.switched ? (
                      <Badge variant="destructive">{t("cert.quota.switched")}</Badge>
                    ) : (
                      <Badge variant="secondary">{t("cert.quota.normal")}</Badge>
                    )}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
      <p className="text-xs text-muted-foreground">
        {t("cert.quota.subtitle", { threshold: thresholdPct })}
      </p>
    </div>
  )
}
