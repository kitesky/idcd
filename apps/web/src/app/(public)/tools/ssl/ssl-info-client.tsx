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
        const rows: [string, string][] = [
          [t("probe.result.rows.domain"), result.domain],
          [t("probe.result.rows.issuer"), result.issuer],
          [t("probe.result.rows.validFrom"), result.valid_from],
          [t("probe.result.rows.validTo"), result.valid_to],
          [
            t("probe.result.rows.certStatus"),
            result.is_valid
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
                  variant={result.days_remaining > 30 ? "default" : "destructive"}
                >
                  {result.days_remaining > 0
                    ? t("probe.result.expiresInDays", {
                        days: result.days_remaining,
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
