"use client"

import { useTranslations } from "next-intl"
import { Card, CardContent, CardHeader, CardTitle, Badge } from "@/components/ui"
import { ToolQueryLayout } from "@/components/tools/ToolQueryLayout"
import { getWhoisInfo, type WhoisInfo } from "@/lib/api"

export default function WhoisInfoClient() {
  const t = useTranslations("tools.whois")
  const helpTips = t.raw("probe.helpTips") as string[]

  return (
    <ToolQueryLayout<WhoisInfo>
      title={t("title")}
      description={t("description")}
      inputLabel={t("probe.label")}
      inputPlaceholder={t("probe.placeholder")}
      inputId="whois-query"
      onQuery={getWhoisInfo}
      renderResult={(result) => {
        const rows: [string, string | undefined][] = [
          [t("probe.result.rows.domain"), result.domain],
          [t("probe.result.rows.registrar"), result.registrar],
          [t("probe.result.rows.createdAt"), result.creation_date],
          [
            t("probe.result.rows.expiresAt"),
            result.expiry_date ?? result.expiration_date,
          ],
        ]
        return (
          <Card>
            <CardHeader>
              <CardTitle>{t("probe.result.title")}</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3 text-sm">
              {rows.map(([k, v]) => (
                <div key={k} className="flex gap-2">
                  <span className="text-muted-foreground w-24 shrink-0 font-medium">
                    {k}
                  </span>
                  <span className="font-mono break-all">{v || "-"}</span>
                </div>
              ))}

              {result.name_servers && result.name_servers.length > 0 && (
                <div className="flex gap-2 flex-wrap items-start">
                  <span className="text-muted-foreground w-24 shrink-0 font-medium">
                    {t("probe.result.rows.nameServers")}
                  </span>
                  <div className="flex gap-1 flex-wrap">
                    {result.name_servers.map((ns) => (
                      <Badge
                        key={ns}
                        variant="secondary"
                        className="font-mono text-xs"
                      >
                        {ns}
                      </Badge>
                    ))}
                  </div>
                </div>
              )}

              {result.note && (
                <div className="flex gap-2">
                  <span className="text-muted-foreground w-24 shrink-0 font-medium">
                    {t("probe.result.rows.note")}
                  </span>
                  <span className="text-muted-foreground break-all">
                    {result.note}
                  </span>
                </div>
              )}
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
