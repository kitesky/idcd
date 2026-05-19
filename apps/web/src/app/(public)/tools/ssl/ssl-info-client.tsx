"use client"

import { useTranslations } from "next-intl"
import { Card, CardContent, CardHeader, CardTitle, Badge } from "@/components/ui"
import { ToolQueryLayout } from "@/components/tools/ToolQueryLayout"
import { getSSLInfo, type SSLInfo } from "@/lib/api"

export default function SslInfoClient() {
  const t = useTranslations("tools.ssl")
  const helpTips = t.raw("probe.helpTips") as string[]

  return (
    <ToolQueryLayout<SSLInfo>
      title={t("title")}
      description={t("description")}
      inputLabel={t("probe.label")}
      inputPlaceholder={t("probe.placeholder")}
      inputId="ssl-query"
      actionLabel={t("probe.actionLabel")}
      loadingLabel={t("probe.loadingLabel")}
      onQuery={getSSLInfo}
      renderResult={(result) => {
        const days = result.days_until_expiry
        const rows: [string, string][] = [
          [t("probe.result.rows.domain"), result.domain],
          [t("probe.result.rows.issuer"), result.issuer],
          [t("probe.result.rows.validFrom"), result.not_before],
          [t("probe.result.rows.validTo"), result.not_after],
          [
            t("probe.result.rows.certStatus"),
            days > 0
              ? t("probe.result.status.valid")
              : t("probe.result.status.invalid"),
          ],
        ]
        return (
          <Card>
            <CardHeader>
              <div className="flex items-center justify-between">
                <CardTitle>{t("probe.result.title")}</CardTitle>
                <Badge
                  variant={days > 30 ? "default" : "destructive"}
                >
                  {days > 0
                    ? t("probe.result.expiresInDays", {
                        days,
                      })
                    : t("probe.result.expired")}
                </Badge>
              </div>
            </CardHeader>
            <CardContent className="space-y-3 text-sm">
              {rows.map(([k, v]) => (
                <div key={k} className="flex gap-2">
                  <span className="text-muted-foreground w-24 shrink-0 font-medium">
                    {k}
                  </span>
                  <span className="font-mono break-all">{v ?? "-"}</span>
                </div>
              ))}
            </CardContent>
          </Card>
        )
      }}
      tips={
        <>
          {helpTips.map((tip, i) => (
            <p key={i}>• {tip}</p>
          ))}
        </>
      }
    />
  )
}
